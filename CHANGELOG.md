# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/).

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
