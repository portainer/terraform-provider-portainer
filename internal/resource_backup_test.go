package internal

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// TestBackupCreate_HappyPath verifies that resourceBackup posts the password
// to /backup and writes the streamed response body to output_path on disk.
func TestBackupCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	expected := []byte("\x1f\x8b\x08fake-tarball-bytes")

	mock.On("POST", "/backup", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(expected)
	})

	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "backup.tar.gz")

	r := resourceBackup()
	d := r.TestResourceData()
	_ = d.Set("password", "topsecret")
	_ = d.Set("output_path", outPath)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() == "" {
		t.Error("expected non-empty ID after backup")
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read backup file: %v", err)
	}
	if string(got) != string(expected) {
		t.Errorf("backup file contents mismatch: got %q want %q", got, expected)
	}

	// Verify request payload carried the password.
	req := mock.FindRequest("POST", "/backup")
	if req == nil {
		t.Fatal("expected POST /backup")
	}
	var payload map[string]interface{}
	if err := req.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if got := payload["password"]; got != "topsecret" {
		t.Errorf("payload.password: got %v", got)
	}
}

// TestBackupCreate_HTTPError verifies that a server error is surfaced and no
// file is written.
func TestBackupCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/backup", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"backup failed"}`,
	))

	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "should-not-exist.tar.gz")

	r := resourceBackup()
	d := r.TestResourceData()
	_ = d.Set("password", "x")
	_ = d.Set("output_path", outPath)

	err := rcCreate(r, d, mock.Client())
	if err == nil {
		t.Fatal("expected error on 500, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
	if _, statErr := os.Stat(outPath); statErr == nil {
		t.Error("expected output file to NOT exist when create fails before file write")
	}
}
