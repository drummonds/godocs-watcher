# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

### Added
- Scanner integration: poll a SANE document scanner (e.g. Epson FF-680W via
  `epsonds`) and auto-scan whenever paper is loaded in the ADF
- Multi-page PDF assembly in pure Go (JPEG pages embedded via DCTDecode,
  no external tools)
- `scanner:` config section (device, source, resolution, mode,
  poll_interval, page_height_mm)
- `-scan` flag for a one-shot scan into `watch_dir`
- Blank-page removal (`skip_blank_pages`, on by default) so duplex scans of
  single-sided documents don't produce empty PDF pages

### Fixed
- Removed the `replace` directive for godocs-client so
  `go install git.bytestone.uk/hum3/godocs-watcher@latest` works

## [0.1.0] - 2026-02-28

### Added
- Directory watching via fsnotify
- Configurable settle time before upload
- Upload to godocs via godocs-client
- Duplicate detection (409 handling)
- Optional delete after upload
- Scan existing files on startup
- Graceful shutdown on SIGINT/SIGTERM
- `-init` flag to generate example config
