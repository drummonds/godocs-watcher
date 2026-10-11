package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

// writePageJPEG creates a simulated A4-ish scan: near-white paper with mild
// noise, plus optional black content rectangles.
func writePageJPEG(t *testing.T, path string, w, h int, content []image.Rectangle) {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = uint8(235 + i%18) // paper white with scanner noise
	}
	for _, r := range content {
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				img.SetGray(x, y, color.Gray{Y: 20})
			}
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestBlankPageDetection(t *testing.T) {
	dir := t.TempDir()
	const w, h = 850, 1100
	threshold := defaultScannerConfig().BlankInkPct / 100

	tests := []struct {
		name    string
		content []image.Rectangle
		blank   bool
	}{
		{"empty page", nil, true},
		{"feed shadow at edge only", []image.Rectangle{image.Rect(0, 0, w, 8)}, true},
		{"full text page", []image.Rectangle{image.Rect(100, 100, 750, 1000)}, false},
		{"single line of text", []image.Rectangle{image.Rect(100, 500, 500, 515)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(dir, "page.jpg")
			writePageJPEG(t, path, w, h, tt.content)
			frac, err := pageInkFraction(path)
			if err != nil {
				t.Fatal(err)
			}
			if got := frac*100 < defaultScannerConfig().BlankInkPct; got != tt.blank {
				t.Errorf("ink=%.4f%% threshold=%.2f%%: blank=%v, want %v",
					frac*100, threshold*100, got, tt.blank)
			}
		})
	}
}
