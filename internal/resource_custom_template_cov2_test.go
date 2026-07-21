package internal

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// =========================================================================
// Additional coverage (cov2) for resource_custom_template.go targeting paths
// not covered by resource_custom_template_test.go / _cov_test.go: the
// file_path create branch (happy + read error), the list-error branch of
// findExistingCustomTemplateByTitle, the file_path Update branch, and the
// repository create path without authentication.
// =========================================================================

// TestCustomTemplateCov2_Create_FromFilePath covers the file_path create
// branch: the file is read, file_content is populated, and the string-create
// endpoint is invoked.
func TestCustomTemplateCov2_Create_FromFilePath(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "compose.yml")
	if err := os.WriteFile(fp, []byte("version: '3'\nservices: {}\n"), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	mock := NewMockServer(t)
	mock.On("GET", "/custom_templates", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/custom_templates/create/string", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":    123,
		"Title": "fromfile",
	}))

	r := resourceCustomTemplate()
	d := r.TestResourceData()
	_ = d.Set("title", "fromfile")
	_ = d.Set("description", "d")
	_ = d.Set("note", "n")
	_ = d.Set("platform", 1)
	_ = d.Set("type", 1)
	_ = d.Set("file_path", fp)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "123" {
		t.Errorf("expected ID 123, got %q", d.Id())
	}
	if got := d.Get("file_content"); got != "version: '3'\nservices: {}\n" {
		t.Errorf("file_content not populated from file, got %q", got)
	}
	if mock.FindRequest("POST", "/custom_templates/create/string") == nil {
		t.Error("expected POST /custom_templates/create/string")
	}
}

// TestCustomTemplateCov2_Create_FromFilePath_ReadError covers the file-read
// error branch when file_path points to a nonexistent file.
func TestCustomTemplateCov2_Create_FromFilePath_ReadError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/custom_templates", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	r := resourceCustomTemplate()
	d := r.TestResourceData()
	_ = d.Set("title", "badfile")
	_ = d.Set("description", "d")
	_ = d.Set("note", "n")
	_ = d.Set("platform", 1)
	_ = d.Set("type", 1)
	_ = d.Set("file_path", "/nonexistent/path/does/not/exist.yml")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error reading nonexistent file, got nil")
	}
}

// TestCustomTemplateCov2_Create_ListError covers the error branch of
// findExistingCustomTemplateByTitle (the initial list call fails).
func TestCustomTemplateCov2_Create_ListError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/custom_templates", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourceCustomTemplate()
	d := r.TestResourceData()
	_ = d.Set("title", "x")
	_ = d.Set("description", "d")
	_ = d.Set("note", "n")
	_ = d.Set("platform", 1)
	_ = d.Set("type", 1)
	_ = d.Set("file_content", "x")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when template list fails, got nil")
	}
}

// TestCustomTemplateCov2_Update_FromFilePath covers the file_path branch of
// Update: the file is read and file_content is populated before the PUT. No
// repository_url, so git_fetch is not called.
func TestCustomTemplateCov2_Update_FromFilePath(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "c.yml")
	if err := os.WriteFile(fp, []byte("version: '3'"), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	mock := NewMockServer(t)
	mock.On("PUT", "/custom_templates/50", RespondString(http.StatusOK, "application/json", `{}`))
	mock.On("GET", "/custom_templates/50", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 50, "Title": "upd", "Platform": 1, "Type": 1,
	}))

	r := resourceCustomTemplate()
	d := r.TestResourceData()
	d.SetId("50")
	_ = d.Set("title", "upd")
	_ = d.Set("description", "d")
	_ = d.Set("note", "n")
	_ = d.Set("platform", 1)
	_ = d.Set("type", 1)
	_ = d.Set("file_path", fp)

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if got := d.Get("file_content"); got != "version: '3'" {
		t.Errorf("file_content not populated from file, got %q", got)
	}
	if mock.FindRequest("PUT", "/custom_templates/50/git_fetch") != nil {
		t.Error("did not expect git_fetch for a non-git template")
	}
}

// TestCustomTemplateCov2_Create_FromRepository_NoAuth covers the repository
// create path with repository_authentication defaulting to false (the
// username/password branch is skipped).
func TestCustomTemplateCov2_Create_FromRepository_NoAuth(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/custom_templates", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/custom_templates/create/repository", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":    66,
		"Title": "repo-noauth",
	}))

	r := resourceCustomTemplate()
	d := r.TestResourceData()
	_ = d.Set("title", "repo-noauth")
	_ = d.Set("description", "d")
	_ = d.Set("note", "n")
	_ = d.Set("platform", 1)
	_ = d.Set("type", 1)
	_ = d.Set("repository_url", "https://github.com/acme/tmpl.git")
	// repository_authentication defaults to false.

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "66" {
		t.Errorf("expected ID 66, got %q", d.Id())
	}
	if mock.FindRequest("POST", "/custom_templates/create/repository") == nil {
		t.Error("expected POST /custom_templates/create/repository")
	}
}
