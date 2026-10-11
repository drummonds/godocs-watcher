package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"
)

// ScannerConfig holds the optional scanner section of the YAML configuration.
type ScannerConfig struct {
	Enabled        bool    `yaml:"enabled"`
	Device         string  `yaml:"device"`
	Source         string  `yaml:"source"`
	Resolution     int     `yaml:"resolution"`
	Mode           string  `yaml:"mode"`
	PollInterval   string  `yaml:"poll_interval"`
	PageHeightMM   float64 `yaml:"page_height_mm"`
	SkipBlankPages bool    `yaml:"skip_blank_pages"`
	BlankInkPct    float64 `yaml:"blank_ink_pct"`
}

func defaultScannerConfig() ScannerConfig {
	return ScannerConfig{
		Source:         "ADF Front",
		Resolution:     300,
		Mode:           "Color",
		PollInterval:   "10s",
		PageHeightMM:   297,
		SkipBlankPages: true,
		BlankInkPct:    0.2,
	}
}

// scanimage exit status when the document feeder is empty (SANE_STATUS_NO_DOCS)
const exitNoDocs = 7

// runScan drives scanimage in batch mode, assembles the scanned pages into a
// multi-page PDF in outDir, and returns the PDF path and page count. If the
// feeder is empty it returns ("", 0, nil).
func runScan(cfg ScannerConfig, outDir string) (string, int, error) {
	tmpDir, err := os.MkdirTemp("", "godocs-scan-")
	if err != nil {
		return "", 0, err
	}
	defer os.RemoveAll(tmpDir)

	args := []string{
		"-d", cfg.Device,
		"--source", cfg.Source,
		"--resolution", fmt.Sprintf("%d", cfg.Resolution),
		"--mode", cfg.Mode,
		"--adf-crp=yes",
		"--format=jpeg",
		"--batch=" + filepath.Join(tmpDir, "page-%04d.jpg"),
	}
	if cfg.PageHeightMM > 0 {
		args = append(args, "-y", fmt.Sprintf("%g", cfg.PageHeightMM))
	}

	cmd := exec.Command("scanimage", args...)
	output, err := cmd.CombinedOutput()

	pages, globErr := filepath.Glob(filepath.Join(tmpDir, "page-*.jpg"))
	if globErr != nil {
		return "", 0, globErr
	}
	sort.Strings(pages)

	if len(pages) == 0 {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == exitNoDocs {
			return "", 0, nil // feeder empty — not an error
		}
		if err != nil {
			return "", 0, fmt.Errorf("scanimage: %w: %s", err, output)
		}
		return "", 0, nil
	}
	// Pages were scanned; a non-zero exit (e.g. paper jam mid-batch) still
	// yields a usable partial document, so log it and carry on.
	if err != nil {
		log.Printf("scan warning (keeping %d scanned pages): %v: %s", len(pages), err, output)
	}

	if cfg.SkipBlankPages {
		kept := pages[:0]
		for _, p := range pages {
			frac, err := pageInkFraction(p)
			if err != nil {
				log.Printf("blank check failed for %s (keeping page): %v", filepath.Base(p), err)
				kept = append(kept, p)
				continue
			}
			if frac*100 < cfg.BlankInkPct {
				log.Printf("dropping blank page (%.3f%% ink)", frac*100)
				continue
			}
			kept = append(kept, p)
		}
		if len(kept) == 0 {
			log.Printf("all %d scanned page(s) were blank — nothing to upload", len(pages))
			return "", 0, nil
		}
		pages = kept
	}

	name := "scan-" + time.Now().Format("20060102-150405") + ".pdf"
	finalPath := filepath.Join(outDir, name)
	tmpPath := filepath.Join(outDir, "."+name+".tmp")

	f, err := os.Create(tmpPath)
	if err != nil {
		return "", 0, err
	}
	if err := writeJPEGPDF(f, pages, cfg.Resolution); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return "", 0, err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return "", 0, err
	}
	// Atomic rename so the watcher only ever sees a complete PDF
	if err := os.Rename(tmpPath, finalPath); err != nil {
		os.Remove(tmpPath)
		return "", 0, err
	}
	return finalPath, len(pages), nil
}

// scanLoop polls the document feeder and scans whenever paper is present.
// Loading a stack into the ADF is the trigger — the scanner's physical
// button is not exposed by the epsonds SANE backend.
func scanLoop(cfg ScannerConfig, outDir string, interval time.Duration, done <-chan struct{}) {
	for {
		select {
		case <-done:
			return
		case <-time.After(interval):
		}

		pdf, pages, err := runScan(cfg, outDir)
		if err != nil {
			log.Printf("scan error: %v", err)
			continue
		}
		if pages > 0 {
			log.Printf("scanned %d page(s) → %s", pages, filepath.Base(pdf))
		}
	}
}
