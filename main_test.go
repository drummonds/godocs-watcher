package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	godocsclient "codeberg.org/hum3/godocs-client"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")

	content := `godocs_server: http://localhost:8000
watch_dir: /tmp/watch
settle_time: 10s
delete_after_upload: true
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.GodocsServer != "http://localhost:8000" {
		t.Errorf("GodocsServer = %q, want %q", cfg.GodocsServer, "http://localhost:8000")
	}
	if cfg.WatchDir != "/tmp/watch" {
		t.Errorf("WatchDir = %q, want %q", cfg.WatchDir, "/tmp/watch")
	}
	if cfg.SettleTime != "10s" {
		t.Errorf("SettleTime = %q, want %q", cfg.SettleTime, "10s")
	}
	if !cfg.DeleteAfterUpload {
		t.Error("DeleteAfterUpload = false, want true")
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")

	content := `godocs_server: http://test:8000
watch_dir: /tmp/w
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.SettleTime != "30s" {
		t.Errorf("SettleTime = %q, want default %q", cfg.SettleTime, "30s")
	}
	if cfg.DeleteAfterUpload {
		t.Error("DeleteAfterUpload should default to false")
	}
}

func TestWriteExampleConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "example.yaml")

	if err := writeExampleConfig(path); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.GodocsServer != "http://localhost:8000" {
		t.Errorf("GodocsServer = %q", cfg.GodocsServer)
	}
	if cfg.SettleTime != "30s" {
		t.Errorf("SettleTime = %q", cfg.SettleTime)
	}
}

func TestShouldSkip(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/tmp/file.pdf", false},
		{"/tmp/.hidden", true},
		{"/tmp/.DS_Store", true},
		{"report.pdf", false},
	}
	for _, tt := range tests {
		if got := shouldSkip(tt.path); got != tt.want {
			t.Errorf("shouldSkip(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestScanExisting(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.pdf"), []byte("pdf"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("txt"), 0644)
	os.WriteFile(filepath.Join(dir, ".hidden"), []byte("no"), 0644)
	os.Mkdir(filepath.Join(dir, "subdir"), 0755)

	pending := make(map[string]time.Time)
	scanExisting(dir, pending)

	if len(pending) != 2 {
		t.Errorf("len(pending) = %d, want 2", len(pending))
	}
	if _, ok := pending[filepath.Join(dir, "a.pdf")]; !ok {
		t.Error("a.pdf not in pending")
	}
	if _, ok := pending[filepath.Join(dir, "b.txt")]; !ok {
		t.Error("b.txt not in pending")
	}
}

func TestUpload(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/document/upload" {
			http.NotFound(w, r)
			return
		}
		r.ParseMultipartForm(10 << 20)
		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		file.Close()
		gotPath = header.Filename

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ulid": "01ABC",
			"name": header.Filename,
		})
	}))
	defer srv.Close()

	// Create a temp file to upload
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.pdf")
	os.WriteFile(testFile, []byte("fake pdf content"), 0644)

	client := godocsclient.NewClient(srv.URL)
	upload(testFile, client, false)

	if gotPath != "test.pdf" {
		t.Errorf("uploaded filename = %q, want %q", gotPath, "test.pdf")
	}

	// File should still exist (deleteAfter=false)
	if _, err := os.Stat(testFile); err != nil {
		t.Error("file was deleted but deleteAfter=false")
	}
}

func TestUploadWithDelete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseMultipartForm(10 << 20)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ulid": "01ABC",
			"name": "del.pdf",
		})
	}))
	defer srv.Close()

	dir := t.TempDir()
	testFile := filepath.Join(dir, "del.pdf")
	os.WriteFile(testFile, []byte("content"), 0644)

	client := godocsclient.NewClient(srv.URL)
	upload(testFile, client, true)

	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("file should have been deleted after upload")
	}
}

func TestUploadDuplicate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseMultipartForm(10 << 20)
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ulid": "01EXIST",
			"name": "dup.pdf",
		})
	}))
	defer srv.Close()

	dir := t.TempDir()
	testFile := filepath.Join(dir, "dup.pdf")
	os.WriteFile(testFile, []byte("content"), 0644)

	client := godocsclient.NewClient(srv.URL)
	// Should not crash on duplicate
	upload(testFile, client, false)
}

func TestCheckSettled(t *testing.T) {
	var uploaded []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseMultipartForm(10 << 20)
		_, header, _ := r.FormFile("file")
		uploaded = append(uploaded, header.Filename)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"ulid": "01X", "name": header.Filename})
	}))
	defer srv.Close()

	dir := t.TempDir()
	settled := filepath.Join(dir, "old.pdf")
	fresh := filepath.Join(dir, "new.pdf")
	os.WriteFile(settled, []byte("old"), 0644)
	os.WriteFile(fresh, []byte("new"), 0644)

	// Backdate the settled file's modtime so re-stat passes
	oldTime := time.Now().Add(-2 * time.Minute)
	os.Chtimes(settled, oldTime, oldTime)

	client := godocsclient.NewClient(srv.URL)
	pending := map[string]time.Time{
		settled: oldTime,    // settled
		fresh:   time.Now(), // not settled
	}

	checkSettled(pending, 30*time.Second, client, false)

	if len(uploaded) != 1 || uploaded[0] != "old.pdf" {
		t.Errorf("uploaded = %v, want [old.pdf]", uploaded)
	}
	if _, ok := pending[settled]; ok {
		t.Error("settled file should be removed from pending")
	}
	if _, ok := pending[fresh]; !ok {
		t.Error("fresh file should remain in pending")
	}
}
