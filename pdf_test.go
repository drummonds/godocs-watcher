package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestJPEG(t *testing.T, path string, w, h int, gray bool) {
	t.Helper()
	var img image.Image
	if gray {
		g := image.NewGray(image.Rect(0, 0, w, h))
		for i := range g.Pix {
			g.Pix[i] = uint8(i % 256)
		}
		img = g
	} else {
		m := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				m.Set(x, y, color.RGBA{uint8(x), uint8(y), 128, 255})
			}
		}
		img = m
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestParseJPEG(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jpg")
	writeTestJPEG(t, path, 123, 45, false)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := parseJPEG(data)
	if err != nil {
		t.Fatal(err)
	}
	if info.width != 123 || info.height != 45 {
		t.Errorf("got %dx%d, want 123x45", info.width, info.height)
	}
	if info.components != 3 {
		t.Errorf("got %d components, want 3", info.components)
	}
}

func TestParseJPEGGray(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gray.jpg")
	writeTestJPEG(t, path, 50, 60, true)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := parseJPEG(data)
	if err != nil {
		t.Fatal(err)
	}
	if info.width != 50 || info.height != 60 || info.components != 1 {
		t.Errorf("got %dx%d comp=%d, want 50x60 comp=1", info.width, info.height, info.components)
	}
}

func TestParseJPEGRejectsNonJPEG(t *testing.T) {
	if _, err := parseJPEG([]byte("not a jpeg at all")); err == nil {
		t.Error("expected error for non-JPEG data")
	}
}

func TestWriteJPEGPDF(t *testing.T) {
	dir := t.TempDir()
	p1 := filepath.Join(dir, "p1.jpg")
	p2 := filepath.Join(dir, "p2.jpg")
	writeTestJPEG(t, p1, 100, 150, false)
	writeTestJPEG(t, p2, 80, 120, true)

	var buf bytes.Buffer
	if err := writeJPEGPDF(&buf, []string{p1, p2}, 300); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	if !strings.HasPrefix(out, "%PDF-1.4") {
		t.Error("missing PDF header")
	}
	if !strings.HasSuffix(out, "%%EOF\n") {
		t.Error("missing EOF marker")
	}
	if !strings.Contains(out, "/Count 2") {
		t.Error("expected 2-page document")
	}
	if strings.Count(out, "/DCTDecode") != 2 {
		t.Error("expected 2 embedded JPEGs")
	}
	if !strings.Contains(out, "/DeviceRGB") || !strings.Contains(out, "/DeviceGray") {
		t.Error("expected both RGB and Gray colour spaces")
	}
	// 100px at 300dpi = 24pt
	if !strings.Contains(out, "/MediaBox [0 0 24.00 36.00]") {
		t.Error("expected 24x36pt MediaBox for 100x150px page at 300dpi")
	}
}

func TestWriteJPEGPDFEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := writeJPEGPDF(&buf, nil, 300); err == nil {
		t.Error("expected error for empty page list")
	}
}
