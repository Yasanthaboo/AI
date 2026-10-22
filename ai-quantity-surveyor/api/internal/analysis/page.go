package analysis

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/image/draw"
)

const maxPageBytes = 10 << 20
const maxPageEdge = 3000
const maxDecodedPixels = 32000000

type PageMetadata struct {
	PageCount   int    `json:"pageCount"`
	ContentType string `json:"contentType"`
}

type PreparedPage struct {
	Content []byte
	Source  Source
}

type PreparationError struct {
	Code      string
	Message   string
	Retryable bool
}

func (failure *PreparationError) Error() string { return failure.Code + ": " + failure.Message }

type PagePreparer struct {
	TempRoot string
	run      func(context.Context, string, ...string) ([]byte, error)
}

func NewPagePreparer() *PagePreparer { return &PagePreparer{run: runPageCommand} }

func (preparer *PagePreparer) Metadata(ctx context.Context, content []byte, fileName string) (PageMetadata, error) {
	if err := checkPageContent(content); err != nil {
		return PageMetadata{}, err
	}
	if !strings.EqualFold(filepath.Ext(fileName), ".pdf") {
		config, format, err := image.DecodeConfig(bytes.NewReader(content))
		if err != nil || (format != "png" && format != "jpeg") || !safeImageDimensions(config.Width, config.Height) {
			return PageMetadata{}, pageFailure("PreparationFailed", "unreadable or oversized image", false)
		}
		contentType := "image/png"
		if format == "jpeg" {
			contentType = "image/jpeg"
		}
		return PageMetadata{PageCount: 1, ContentType: contentType}, nil
	}
	directory, err := preparer.writePDF(content)
	if err != nil {
		return PageMetadata{}, err
	}
	defer os.RemoveAll(directory)
	return preparer.pdfMetadata(ctx, filepath.Join(directory, "drawing.pdf"))
}

func (preparer *PagePreparer) Prepare(ctx context.Context, content []byte, fileName string, pageNumber int, selectionMode string) (PreparedPage, error) {
	if err := checkPageContent(content); err != nil {
		return PreparedPage{}, err
	}
	if pageNumber < 1 {
		return PreparedPage{}, pageFailure("InvalidSourcePage", "page number must be positive", false)
	}
	if err := ctx.Err(); err != nil {
		return PreparedPage{}, pageFailure("PreparationFailed", err.Error(), true)
	}
	imageContent := content
	if strings.EqualFold(filepath.Ext(fileName), ".pdf") {
		if selectionMode != "explicit" && selectionMode != "legacy-default" {
			return PreparedPage{}, pageFailure("InvalidSourcePage", "PDF selection mode is required", false)
		}
		directory, err := preparer.writePDF(content)
		if err != nil {
			return PreparedPage{}, err
		}
		defer os.RemoveAll(directory)
		input := filepath.Join(directory, "drawing.pdf")
		metadata, err := preparer.pdfMetadata(ctx, input)
		if err != nil {
			return PreparedPage{}, err
		}
		if pageNumber > metadata.PageCount {
			return PreparedPage{}, pageFailure("InvalidSourcePage", "selected page exceeds drawing page count", false)
		}
		output := filepath.Join(directory, "page")
		renderContext, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		_, err = preparer.run(renderContext, "pdftoppm", "-f", strconv.Itoa(pageNumber), "-l", strconv.Itoa(pageNumber), "-singlefile", "-cropbox", "-scale-to", strconv.Itoa(maxPageEdge), "-png", input, output)
		if err != nil {
			return PreparedPage{}, pageFailure("PreparationFailed", "selected PDF page could not be rendered", true)
		}
		file, err := os.Open(output + ".png")
		if err != nil {
			return PreparedPage{}, pageFailure("PreparationFailed", "rendered page unavailable", true)
		}
		imageContent, err = io.ReadAll(io.LimitReader(file, maxPageBytes+1))
		closeErr := file.Close()
		if err != nil || closeErr != nil || len(imageContent) > maxPageBytes {
			return PreparedPage{}, pageFailure("PreparationFailed", "rendered page exceeds output limit", false)
		}
	} else {
		if pageNumber != 1 {
			return PreparedPage{}, pageFailure("InvalidSourcePage", "images only support page 1", false)
		}
		selectionMode = "image"
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(imageContent))
	if err != nil || (format != "png" && format != "jpeg") || !safeImageDimensions(config.Width, config.Height) {
		return PreparedPage{}, pageFailure("PreparationFailed", "unreadable or oversized prepared image", false)
	}
	decoded, _, err := image.Decode(bytes.NewReader(imageContent))
	if err != nil {
		return PreparedPage{}, pageFailure("PreparationFailed", "prepared image cannot be decoded", false)
	}
	width, height := config.Width, config.Height
	longEdge := max(width, height)
	if longEdge > maxPageEdge {
		width = max(1, width*maxPageEdge/longEdge)
		height = max(1, height*maxPageEdge/longEdge)
		resized := image.NewRGBA(image.Rect(0, 0, width, height))
		draw.ApproxBiLinear.Scale(resized, resized.Bounds(), decoded, decoded.Bounds(), draw.Src, nil)
		decoded = resized
	}
	var encoded boundedPageBuffer
	encoded.limit = maxPageBytes
	if err := png.Encode(&encoded, decoded); err != nil {
		return PreparedPage{}, pageFailure("PreparationFailed", "canonical PNG exceeds output limit", false)
	}
	if err := ctx.Err(); err != nil {
		return PreparedPage{}, pageFailure("PreparationFailed", err.Error(), true)
	}
	digest := sha256.Sum256(encoded.Bytes())
	return PreparedPage{Content: encoded.Bytes(), Source: Source{PageNumber: pageNumber, SelectionMode: selectionMode, ImageDigest: hex.EncodeToString(digest[:]), PixelWidth: width, PixelHeight: height}}, nil
}

func (preparer *PagePreparer) writePDF(content []byte) (string, error) {
	if !bytes.HasPrefix(content, []byte("%PDF-")) {
		return "", pageFailure("PreparationFailed", "invalid PDF signature", false)
	}
	directory, err := os.MkdirTemp(preparer.TempRoot, "surveyor-page-")
	if err != nil {
		return "", pageFailure("PreparationFailed", "temporary storage unavailable", true)
	}
	if err := os.WriteFile(filepath.Join(directory, "drawing.pdf"), content, 0600); err != nil {
		os.RemoveAll(directory)
		return "", pageFailure("PreparationFailed", "temporary drawing write failed", true)
	}
	return directory, nil
}

func (preparer *PagePreparer) pdfMetadata(ctx context.Context, input string) (PageMetadata, error) {
	metadataContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	output, err := preparer.run(metadataContext, "pdfinfo", input)
	if err != nil {
		return PageMetadata{}, pageFailure("PreparationFailed", "PDF metadata unavailable or encrypted", false)
	}
	count, found := 0, false
	for _, line := range strings.Split(string(output), "\n") {
		key, value, exists := strings.Cut(line, ":")
		if exists && strings.TrimSpace(key) == "Encrypted" && strings.HasPrefix(strings.TrimSpace(value), "yes") {
			return PageMetadata{}, pageFailure("PreparationFailed", "encrypted PDFs are not supported", false)
		}
		if !exists || strings.TrimSpace(key) != "Pages" {
			continue
		}
		if found {
			return PageMetadata{}, pageFailure("PreparationFailed", "ambiguous PDF page count", false)
		}
		found = true
		count, err = strconv.Atoi(strings.TrimSpace(value))
		if err != nil || count < 1 {
			return PageMetadata{}, pageFailure("PreparationFailed", "invalid PDF page count", false)
		}
	}
	if !found {
		return PageMetadata{}, pageFailure("PreparationFailed", "PDF page count missing", false)
	}
	return PageMetadata{PageCount: count, ContentType: "application/pdf"}, nil
}

func checkPageContent(content []byte) error {
	if len(content) == 0 || len(content) > maxPageBytes {
		return pageFailure("PreparationFailed", "drawing must contain 1 byte to 10 MB", false)
	}
	return nil
}

func safeImageDimensions(width, height int) bool {
	return width > 0 && height > 0 && width <= maxDecodedPixels/height
}

func pageFailure(code, message string, retryable bool) error {
	return &PreparationError{Code: code, Message: message, Retryable: retryable}
}

type boundedPageBuffer struct {
	bytes.Buffer
	limit int
}

func (buffer *boundedPageBuffer) Write(content []byte) (int, error) {
	if len(content) > buffer.limit-buffer.Len() {
		return 0, fmt.Errorf("command output exceeds limit")
	}
	return buffer.Buffer.Write(content)
}

func runPageCommand(ctx context.Context, executable string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, executable, args...)
	stdout := &boundedPageBuffer{limit: 64 << 10}
	stderr := &boundedPageBuffer{limit: 16 << 10}
	command.Stdout, command.Stderr = stdout, stderr
	if err := command.Run(); err != nil {
		return nil, err
	}
	return stdout.Bytes(), nil
}
