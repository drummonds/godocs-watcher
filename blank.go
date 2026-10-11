package main

import (
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"os"
)

const (
	// Fraction of the page edge to ignore — feed shadows and skew wedges
	// live there and would otherwise count as ink.
	blankBorderFrac = 0.04
	// Luminance (0-255) below which a pixel counts as ink. Paper white
	// scans around 230-255; bleed-through from the far side stays lighter
	// than this.
	inkLumaMax = 160
	// Cap on sampled pixels per page, for speed on large scans.
	blankMaxSamples = 400000
)

// pageInkFraction returns the fraction of sampled pixels dark enough to be
// ink, looking only at the page interior.
func pageInkFraction(path string) (float64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	img, err := jpeg.Decode(f)
	if err != nil {
		return 0, err
	}

	b := img.Bounds()
	insetX := int(float64(b.Dx()) * blankBorderFrac)
	insetY := int(float64(b.Dy()) * blankBorderFrac)
	r := image.Rect(b.Min.X+insetX, b.Min.Y+insetY, b.Max.X-insetX, b.Max.Y-insetY)
	if r.Empty() {
		r = b
	}

	step := 1
	if total := r.Dx() * r.Dy(); total > blankMaxSamples {
		step = int(math.Ceil(math.Sqrt(float64(total) / blankMaxSamples)))
	}

	luma := func(x, y int) uint8 {
		switch m := img.(type) {
		case *image.YCbCr:
			return m.Y[m.YOffset(x, y)]
		case *image.Gray:
			return m.Pix[m.PixOffset(x, y)]
		default:
			return color.GrayModel.Convert(img.At(x, y)).(color.Gray).Y
		}
	}

	var samples, ink int
	for y := r.Min.Y; y < r.Max.Y; y += step {
		for x := r.Min.X; x < r.Max.X; x += step {
			samples++
			if luma(x, y) < inkLumaMax {
				ink++
			}
		}
	}
	if samples == 0 {
		return 0, nil
	}
	return float64(ink) / float64(samples), nil
}
