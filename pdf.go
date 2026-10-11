package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
)

// jpegInfo holds the dimensions and colour layout parsed from a JPEG file.
type jpegInfo struct {
	width, height, components int
}

// parseJPEG extracts image dimensions and component count from JPEG data by
// walking the marker segments until a start-of-frame marker is found.
func parseJPEG(data []byte) (jpegInfo, error) {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return jpegInfo{}, fmt.Errorf("not a JPEG file")
	}
	i := 2
	for i+4 <= len(data) {
		if data[i] != 0xFF {
			return jpegInfo{}, fmt.Errorf("invalid JPEG marker at offset %d", i)
		}
		// Skip fill bytes
		for i < len(data) && data[i] == 0xFF {
			i++
		}
		if i >= len(data) {
			break
		}
		marker := data[i]
		i++
		// Standalone markers without a length field
		if marker == 0xD8 || marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7) {
			continue
		}
		if i+2 > len(data) {
			break
		}
		segLen := int(data[i])<<8 | int(data[i+1])
		// SOF0..SOF15 excluding DHT (C4), JPG (C8), DAC (CC)
		if marker >= 0xC0 && marker <= 0xCF && marker != 0xC4 && marker != 0xC8 && marker != 0xCC {
			if i+8 > len(data) {
				return jpegInfo{}, fmt.Errorf("truncated SOF segment")
			}
			return jpegInfo{
				height:     int(data[i+3])<<8 | int(data[i+4]),
				width:      int(data[i+5])<<8 | int(data[i+6]),
				components: int(data[i+7]),
			}, nil
		}
		i += segLen
	}
	return jpegInfo{}, fmt.Errorf("no SOF marker found")
}

// writeJPEGPDF assembles JPEG files into a single multi-page PDF, embedding
// each JPEG unmodified (DCTDecode). dpi maps pixels to page points.
func writeJPEGPDF(w io.Writer, jpegPaths []string, dpi int) error {
	if len(jpegPaths) == 0 {
		return fmt.Errorf("no pages to write")
	}
	if dpi <= 0 {
		return fmt.Errorf("invalid dpi %d", dpi)
	}

	var buf bytes.Buffer
	offsets := []int{0} // object numbers are 1-based; index 0 unused
	addObj := func(body string) int {
		offsets = append(offsets, buf.Len())
		n := len(offsets) - 1
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", n, body)
		return n
	}

	buf.WriteString("%PDF-1.4\n%\xff\xff\xff\xff\n")

	// Objects 1 and 2 are reserved for Catalog and Pages, written after the
	// page objects so the Kids list is known. Reserve their slots now.
	offsets = append(offsets, -1, -1)

	var pageRefs []int
	for _, path := range jpegPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := parseJPEG(data)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		var colorSpace string
		switch info.components {
		case 1:
			colorSpace = "/DeviceGray"
		case 3:
			colorSpace = "/DeviceRGB"
		case 4:
			colorSpace = "/DeviceCMYK"
		default:
			return fmt.Errorf("%s: unsupported JPEG with %d components", path, info.components)
		}

		wPt := float64(info.width) * 72.0 / float64(dpi)
		hPt := float64(info.height) * 72.0 / float64(dpi)

		// Image XObject with the raw JPEG as its stream
		offsets = append(offsets, buf.Len())
		imgObj := len(offsets) - 1
		fmt.Fprintf(&buf, "%d 0 obj\n<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace %s /BitsPerComponent 8 /Filter /DCTDecode /Length %d >>\nstream\n",
			imgObj, info.width, info.height, colorSpace, len(data))
		buf.Write(data)
		buf.WriteString("\nendstream\nendobj\n")

		content := fmt.Sprintf("q %.2f 0 0 %.2f 0 0 cm /Im0 Do Q", wPt, hPt)
		contentObj := addObj(fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content))

		pageObj := addObj(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.2f %.2f] /Resources << /XObject << /Im0 %d 0 R >> >> /Contents %d 0 R >>",
			wPt, hPt, imgObj, contentObj))
		pageRefs = append(pageRefs, pageObj)
	}

	// Now write Catalog (1) and Pages (2) at the current offset
	offsets[1] = buf.Len()
	buf.WriteString("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")
	offsets[2] = buf.Len()
	kids := ""
	for _, r := range pageRefs {
		kids += fmt.Sprintf("%d 0 R ", r)
	}
	fmt.Fprintf(&buf, "2 0 obj\n<< /Type /Pages /Kids [%s] /Count %d >>\nendobj\n", kids, len(pageRefs))

	xrefStart := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, off := range offsets[1:] {
		fmt.Fprintf(&buf, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xrefStart)

	_, err := w.Write(buf.Bytes())
	return err
}
