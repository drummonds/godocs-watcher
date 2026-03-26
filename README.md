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

## Config

| Field | Default | Description |
|---|---|---|
| `godocs_server` | *(required)* | URL of the godocs server |
| `watch_dir` | *(required)* | Directory to watch |
| `settle_time` | `30s` | Duration with no writes before uploading |
| `delete_after_upload` | `false` | Remove source file after successful upload |

## Build

```bash
task build    # build binary
task check    # fmt + vet + test
```

## Links

| | |
|---|---|
| Documentation | https://h3-godocs-watcher.statichost.page/ |
| Source (Codeberg) | https://codeberg.org/hum3/godocs-watcher |
| Mirror (GitHub) | https://github.com/drummonds/godocs-watcher |
