package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	godocsclient "github.com/drummonds/godocs-client"
	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

const configFileName = "godocs-watcher.yaml"

// Config holds the YAML configuration.
type Config struct {
	GodocsServer      string `yaml:"godocs_server"`
	WatchDir          string `yaml:"watch_dir"`
	SettleTime        string `yaml:"settle_time"`
	DeleteAfterUpload bool   `yaml:"delete_after_upload"`
}

func defaultConfig() Config {
	return Config{
		SettleTime: "30s",
	}
}

func loadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	cfg := defaultConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	return cfg, nil
}

func writeExampleConfig(path string) error {
	cfg := Config{
		GodocsServer:      "http://localhost:8000",
		WatchDir:          "/path/to/watch",
		SettleTime:        "30s",
		DeleteAfterUpload: false,
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	header := "# godocs-watcher configuration\n# Watches a directory and uploads settled files to godocs\n\n"
	return os.WriteFile(path, []byte(header+string(data)), 0644)
}

func main() {
	initCfg := flag.Bool("init", false, "Write an example "+configFileName+" and exit")
	flag.Parse()

	if *initCfg {
		if err := writeExampleConfig(configFileName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Wrote %s\n", configFileName)
		return
	}

	cfg, err := loadConfig(configFileName)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "No %s found. Run with -init to create one.\n", configFileName)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if cfg.GodocsServer == "" {
		fmt.Fprintf(os.Stderr, "Error: godocs_server must be set in %s\n", configFileName)
		os.Exit(1)
	}
	if cfg.WatchDir == "" {
		fmt.Fprintf(os.Stderr, "Error: watch_dir must be set in %s\n", configFileName)
		os.Exit(1)
	}

	settleTime, err := time.ParseDuration(cfg.SettleTime)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid settle_time %q: %v\n", cfg.SettleTime, err)
		os.Exit(1)
	}

	// Verify watch directory exists
	info, err := os.Stat(cfg.WatchDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: watch_dir %q: %v\n", cfg.WatchDir, err)
		os.Exit(1)
	}
	if !info.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: watch_dir %q is not a directory\n", cfg.WatchDir)
		os.Exit(1)
	}

	client := godocsclient.NewClient(cfg.GodocsServer)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating watcher: %v\n", err)
		os.Exit(1)
	}
	defer watcher.Close()

	if err := watcher.Add(cfg.WatchDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error watching %s: %v\n", cfg.WatchDir, err)
		os.Exit(1)
	}

	log.Printf("godocs-watcher started")
	log.Printf("  server:     %s", cfg.GodocsServer)
	log.Printf("  watch_dir:  %s", cfg.WatchDir)
	log.Printf("  settle:     %s", settleTime)
	log.Printf("  delete:     %v", cfg.DeleteAfterUpload)

	// Track pending files: path → last modification time
	pending := make(map[string]time.Time)

	// Scan existing files on startup
	scanExisting(cfg.WatchDir, pending)

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if shouldSkip(event.Name) {
				continue
			}
			if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) {
				// Skip directories
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					continue
				}
				pending[event.Name] = time.Now()
				log.Printf("activity: %s", filepath.Base(event.Name))
			}
			if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
				delete(pending, event.Name)
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("watcher error: %v", err)

		case <-ticker.C:
			checkSettled(pending, settleTime, client, cfg.DeleteAfterUpload)

		case <-sigCh:
			log.Println("shutting down")
			return
		}
	}
}

// scanExisting adds all non-dot, non-directory files in dir to pending.
func scanExisting(dir string, pending map[string]time.Time) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("scan error: %v", err)
		return
	}
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		info, err := e.Info()
		if err != nil {
			continue
		}
		pending[path] = info.ModTime()
		log.Printf("existing: %s", e.Name())
	}
}

// shouldSkip returns true for dotfiles and other paths to ignore.
func shouldSkip(path string) bool {
	return strings.HasPrefix(filepath.Base(path), ".")
}

// checkSettled uploads files that have been quiet for settleTime.
func checkSettled(pending map[string]time.Time, settleTime time.Duration, client *godocsclient.Client, deleteAfter bool) {
	now := time.Now()
	for path, lastMod := range pending {
		if now.Sub(lastMod) < settleTime {
			continue
		}

		// Re-stat to catch late writes
		info, err := os.Stat(path)
		if err != nil {
			// File gone — remove from pending
			delete(pending, path)
			continue
		}
		if info.IsDir() {
			delete(pending, path)
			continue
		}
		// If file was modified since we last saw it, update and wait again
		if info.ModTime().After(lastMod) {
			pending[path] = info.ModTime()
			continue
		}

		upload(path, client, deleteAfter)
		delete(pending, path)
	}
}

// upload sends a file to godocs and optionally deletes it.
func upload(path string, client *godocsclient.Client, deleteAfter bool) {
	name := filepath.Base(path)
	log.Printf("uploading: %s", name)

	result, err := client.Upload(path, "")
	if err != nil {
		log.Printf("upload failed: %s: %v", name, err)
		return
	}

	if result.Duplicate {
		log.Printf("duplicate: %s (already in godocs)", name)
	} else {
		log.Printf("uploaded: %s → %s", name, result.ULID)
	}

	if deleteAfter {
		if err := os.Remove(path); err != nil {
			log.Printf("delete failed: %s: %v", name, err)
		} else {
			log.Printf("deleted: %s", name)
		}
	}
}
