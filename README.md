# godocs-watcher

Directory watcher that monitors a folder for new files, waits for them to settle (no writes for a configurable duration), then uploads them to a [godocs](https://github.com/drummonds/godocs) server.

## Install

```bash
go install github.com/drummonds/godocs-watcher@latest
```

Or download from [releases](https://github.com/drummonds/godocs-watcher/releases).

## Usage

Generate a config file:

```bash
godocs-watcher -init
```

Edit `godocs-watcher.yaml`:

```yaml
godocs_server: http://localhost:8000
watch_dir: /home/user/scans
settle_time: 30s
delete_after_upload: false
```

Run:

```bash
godocs-watcher
```

The watcher will:
1. Scan existing files in `watch_dir` on startup
2. Watch for new/modified files via fsnotify
3. Wait until a file has no writes for `settle_time`
4. Upload to godocs via `POST /api/document/upload`
5. Optionally delete the source file after successful upload

Dotfiles and directories are ignored.

## Scanner integration

godocs-watcher can drive a SANE document scanner (tested with an Epson
FastFoto FF-680W over the network via the `epsonds` backend) and feed the
results straight into the upload pipeline. Requires `scanimage` (package
`sane-utils`).

```yaml
scanner:
  enabled: true
  device: epsonds:net:10.0.0.64
  source: ADF Front          # or "ADF Duplex"
  resolution: 300
  mode: Color                # Lineart | Gray | Color
  poll_interval: 10s
  page_height_mm: 297        # A4; 0 = scanner maximum
  skip_blank_pages: true     # drop blank pages (e.g. backs in duplex mode)
  blank_ink_pct: 0.2         # pages with less dark-pixel coverage are blank
```

When enabled, the watcher polls the document feeder every `poll_interval`.
**Loading a stack of paper into the ADF is the trigger** — within one poll
interval the whole stack is scanned, assembled into a single multi-page PDF
(JPEG pages embedded losslessly, no external tools needed), dropped into
`watch_dir`, and uploaded like any other file. OCR happens server-side in
godocs.

The scanner's physical button cannot be used on Linux: the `epsonds` SANE
backend exposes no button/sensor options, and Epson's push-scan only talks
to their Windows/Mac software. Feeder polling replaces it — load paper and
walk away.

A one-shot scan (no daemon) is also available:

```bash
godocs-watcher -scan
```

Find your device name with `scanimage -L`.

## Config

| Field | Default | Description |
|---|---|---|
| `godocs_server` | *(required)* | URL of the godocs server |
| `watch_dir` | *(required)* | Directory to watch |
| `settle_time` | `30s` | Duration with no writes before uploading |
| `delete_after_upload` | `false` | Remove source file after successful upload |
| `scanner.enabled` | `false` | Poll a SANE scanner and auto-scan when paper is loaded |
| `scanner.device` | *(required if enabled)* | SANE device name from `scanimage -L` |
| `scanner.source` | `ADF Front` | `ADF Front` or `ADF Duplex` |
| `scanner.resolution` | `300` | Scan resolution in dpi |
| `scanner.mode` | `Color` | `Lineart`, `Gray` or `Color` |
| `scanner.poll_interval` | `10s` | How often to check the feeder for paper |
| `scanner.page_height_mm` | `297` | Page scan length (A4); `0` for scanner maximum |
| `scanner.skip_blank_pages` | `true` | Drop blank pages before building the PDF |
| `scanner.blank_ink_pct` | `0.2` | Dark-pixel % below which a page counts as blank |

## Build

```bash
task build    # build binary
task check    # fmt + vet + test
```

## Links

| | |
|---|---|
| Source (Forgejo) | https://git.bytestone.uk/hum3/godocs-watcher |
| Mirror (GitHub) | https://github.com/drummonds/godocs-watcher |
