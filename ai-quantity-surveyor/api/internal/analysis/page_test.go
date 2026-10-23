package analysis

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func pageImage(t *testing.T, width, height int) []byte {
	t.Helper()
	var output bytes.Buffer
	if err := png.Encode(&output, image.NewRGBA(image.Rect(0, 0, width, height))); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func TestPrepareImageCanonicalSource(t *testing.T) {
	preparer := NewPagePreparer()
	content := pageImage(t, 4000, 1000)
	page, err := preparer.Prepare(context.Background(), content, "plan.png", 1, "explicit")
	if err != nil {
		t.Fatal(err)
	}
	if page.Source.PixelWidth != 3000 || page.Source.PixelHeight != 750 || page.Source.SelectionMode != "image" || len(page.Source.ImageDigest) != 64 {
		t.Fatalf("unexpected source: %+v", page.Source)
	}
	if _, err := preparer.Prepare(context.Background(), content, "plan.png", 2, "explicit"); err == nil {
		t.Fatal("image accepted page 2")
	}
	metadata, err := preparer.Metadata(context.Background(), content, "plan.png")
	if err != nil || metadata.PageCount != 1 {
		t.Fatalf("image metadata: %+v %v", metadata, err)
	}
}

func TestPrepareSelectedPDFPageAndCleanup(t *testing.T) {
	preparer := NewPagePreparer()
	preparer.TempRoot = t.TempDir()
	rendered := pageImage(t, 200, 400)
	preparer.run = func(ctx context.Context, executable string, args ...string) ([]byte, error) {
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("command has no deadline")
		}
		if executable == "pdfinfo" {
			return []byte("Pages: 3\n"), nil
		}
		if executable != "pdftoppm" || args[0] != "-f" || args[1] != "2" || args[2] != "-l" || args[3] != "2" {
			t.Fatalf("unexpected command: %s %v", executable, args)
		}
		return nil, os.WriteFile(args[len(args)-1]+".png", rendered, 0600)
	}
	page, err := preparer.Prepare(context.Background(), []byte("%PDF-1.4\nfixture"), "plan.pdf", 2, "explicit")
	if err != nil {
		t.Fatal(err)
	}
	if page.Source.PageNumber != 2 || page.Source.PixelHeight != 400 || page.Source.PixelWidth != 200 {
		t.Fatalf("source mapping changed: %+v", page.Source)
	}
	entries, err := os.ReadDir(preparer.TempRoot)
	if err != nil || len(entries) != 0 {
		t.Fatalf("temporary files not removed: %v %v", entries, err)
	}
}

func TestPreparePDFRejectsInvalidPageBeforeRendering(t *testing.T) {
	preparer := NewPagePreparer()
	preparer.TempRoot = t.TempDir()
	preparer.run = func(_ context.Context, executable string, _ ...string) ([]byte, error) {
		if executable != "pdfinfo" {
			t.Fatal("invalid page was rendered")
		}
		return []byte("Pages: 1\n"), nil
	}
	_, err := preparer.Prepare(context.Background(), []byte("%PDF-1.4\nfixture"), "plan.pdf", 2, "explicit")
	var failure *PreparationError
	if !errors.As(err, &failure) || failure.Code != "InvalidSourcePage" {
		t.Fatalf("wrong error: %v", err)
	}
}

func TestPreparePDFCommandFailureCleansUp(t *testing.T) {
	preparer := NewPagePreparer()
	preparer.TempRoot = t.TempDir()
	preparer.run = func(_ context.Context, executable string, _ ...string) ([]byte, error) {
		if executable == "pdfinfo" {
			return []byte("Pages: 2\n"), nil
		}
		return nil, context.DeadlineExceeded
	}
	if _, err := preparer.Prepare(context.Background(), []byte("%PDF-1.4\nfixture"), "plan.pdf", 2, "explicit"); err == nil {
		t.Fatal("render failure accepted")
	}
	entries, err := os.ReadDir(preparer.TempRoot)
	if err != nil || len(entries) != 0 {
		t.Fatalf("temporary files leaked: %v %v", entries, err)
	}
}

func TestPageMetadataRejectsAmbiguousOrEncryptedPDF(t *testing.T) {
	for _, output := range []string{"Pages: 2\nPages: 4\n", "Pages: 0\n", "Title: No page count\n", "Pages: 2\nEncrypted: yes (print:yes)\n"} {
		preparer := NewPagePreparer()
		preparer.TempRoot = t.TempDir()
		preparer.run = func(context.Context, string, ...string) ([]byte, error) { return []byte(output), nil }
		if _, err := preparer.Metadata(context.Background(), []byte("%PDF-1.4\nfixture"), "plan.pdf"); err == nil {
			t.Fatalf("accepted %q", output)
		}
	}
	preparer := NewPagePreparer()
	preparer.TempRoot = t.TempDir()
	preparer.run = func(context.Context, string, ...string) ([]byte, error) { return nil, errors.New("encrypted") }
	if _, err := preparer.Metadata(context.Background(), []byte("%PDF-1.4\nfixture"), "plan.pdf"); err == nil {
		t.Fatal("encrypted PDF accepted")
	}
}

func TestPrepareRejectsOversizedOutputAndCanceledContext(t *testing.T) {
	preparer := NewPagePreparer()
	preparer.TempRoot = t.TempDir()
	preparer.run = func(_ context.Context, executable string, args ...string) ([]byte, error) {
		if executable == "pdfinfo" {
			return []byte("Pages: 1\n"), nil
		}
		return nil, os.WriteFile(filepath.Clean(args[len(args)-1]+".png"), bytes.Repeat([]byte("x"), maxPageBytes+1), 0600)
	}
	if _, err := preparer.Prepare(context.Background(), []byte("%PDF-1.4\nfixture"), "plan.pdf", 1, "explicit"); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("expected output-limit error: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := preparer.Prepare(ctx, []byte("%PDF-1.4\nfixture"), "plan.pdf", 1, "explicit"); err == nil {
		t.Fatal("canceled work started")
	}
	if _, err := preparer.Prepare(context.Background(), make([]byte, maxPageBytes+1), "plan.pdf", 1, "explicit"); err == nil {
		t.Fatal("oversized original accepted")
	}
}

func TestPreparePDFWithPoppler(t *testing.T) {
	if _, err := exec.LookPath("pdfinfo"); err != nil {
		t.Skip("Poppler runtime required")
	}
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		t.Skip("Poppler runtime required")
	}
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R 4 0 R] /Count 2 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 100] /Resources << >> >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 400] /CropBox [0 0 200 300] /Rotate 90 /Resources << >> >>",
	}
	var content bytes.Buffer
	content.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	for index, object := range objects {
		offsets = append(offsets, content.Len())
		fmt.Fprintf(&content, "%d 0 obj\n%s\nendobj\n", index+1, object)
	}
	xref := content.Len()
	fmt.Fprintf(&content, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&content, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&content, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	preparer := NewPagePreparer()
	metadata, err := preparer.Metadata(context.Background(), content.Bytes(), "drawing.pdf")
	if err != nil || metadata.PageCount != 2 {
		t.Fatalf("real metadata: %+v %v", metadata, err)
	}
	page, err := preparer.Prepare(context.Background(), content.Bytes(), "drawing.pdf", 2, "explicit")
	if err != nil {
		t.Fatal(err)
	}
	if page.Source.PageNumber != 2 || page.Source.PixelWidth > maxPageEdge || page.Source.PixelHeight > maxPageEdge {
		t.Fatalf("invalid selected-page bounds: %+v", page.Source)
	}
	if page.Source.PixelWidth != page.Source.PixelHeight*3/2 {
		t.Fatalf("rotation/crop mapping incorrect: %+v", page.Source)
	}
	t.Logf("selected page 2 canonical image: %dx%d, %d bytes", page.Source.PixelWidth, page.Source.PixelHeight, len(page.Content))
}
