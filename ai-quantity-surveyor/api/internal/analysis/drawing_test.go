package analysis

import (
	"bytes"
	"image"
	"image/png"
	"math"
	"testing"
)

func TestSanitizeBoundsNormalizesAndDropsInvalid(t *testing.T) {
	candidate := Candidate{DrawingAspect: 99, Rooms: []Room{
		{ID: "fraction", Bounds: &Bounds{X: 0.1, Y: 0.2, Width: 0.3, Height: 0.4}},
		{ID: "thousand", Bounds: &Bounds{X: 100, Y: 200, Width: 300, Height: 400}},
		{ID: "overflow", Bounds: &Bounds{X: 0.9, Y: 0.9, Width: 0.5, Height: 0.5}},
		{ID: "negative", Bounds: &Bounds{X: -1, Y: 0, Width: 0.2, Height: 0.2}},
		{ID: "nan", Bounds: &Bounds{X: math.NaN(), Y: 0, Width: 0.2, Height: 0.2}},
		{ID: "tiny", Bounds: &Bounds{X: 0.1, Y: 0.1, Width: 0.001, Height: 0.2}},
	}}
	sanitizeBounds(&candidate)
	if got := candidate.Rooms[0].Bounds; got == nil || *got != (Bounds{X: 0.1, Y: 0.2, Width: 0.3, Height: 0.4}) {
		t.Fatalf("fraction bounds changed: %+v", got)
	}
	if got := candidate.Rooms[1].Bounds; got == nil || math.Abs(got.X-0.1) > 1e-9 || math.Abs(got.Height-0.4) > 1e-9 {
		t.Fatalf("0-1000 bounds not rescaled: %+v", got)
	}
	if got := candidate.Rooms[2].Bounds; got == nil || got.X+got.Width > 1+1e-9 || got.Y+got.Height > 1+1e-9 {
		t.Fatalf("overflowing bounds not clipped: %+v", got)
	}
	for _, room := range candidate.Rooms[3:] {
		if room.Bounds != nil {
			t.Fatalf("invalid bounds kept for %s: %+v", room.ID, room.Bounds)
		}
	}
	if candidate.DrawingAspect != 0 {
		t.Fatalf("implausible aspect kept: %v", candidate.DrawingAspect)
	}
}

func TestDrawingAspectReadsImagesAndPDFPageBox(t *testing.T) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewGray(image.Rect(0, 0, 400, 300))); err != nil {
		t.Fatal(err)
	}
	if got := drawingAspect(encoded.Bytes()); math.Abs(got-0.75) > 1e-9 {
		t.Fatalf("png aspect = %v", got)
	}
	if got := drawingAspect([]byte("%PDF-1.7 /Type /Page /MediaBox [0 0 842 595]")); math.Abs(got-595.0/842) > 1e-9 {
		t.Fatalf("pdf aspect = %v", got)
	}
	if got := drawingAspect([]byte("%PDF-1.7 /MediaBox [0 0 842 595] /Rotate 90")); math.Abs(got-842.0/595) > 1e-9 {
		t.Fatalf("rotated pdf aspect = %v", got)
	}
	if got := drawingAspect([]byte("not a drawing")); got != 0 {
		t.Fatalf("unknown aspect = %v", got)
	}
}
