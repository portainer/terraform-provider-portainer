package internal

import (
	"net/http"
	"testing"
)

// TestDataSourceGitopsRepoFileRead_HappyPath verifies the data source POSTs
// to /gitops/repo/file/preview, decodes the FileContent, and constructs a
// deterministic ID.
func TestDataSourceGitopsRepoFileRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/gitops/repo/file/preview", RespondJSON(http.StatusOK, map[string]interface{}{
		"FileContent": "version: '3'\nservices:\n  app:\n    image: nginx\n",
	}))

	ds := dataSourceGitopsRepoFile()
	d := ds.TestResourceData()
	_ = d.Set("repository_url", "https://github.com/owner/repo.git")
	_ = d.Set("reference", "refs/heads/main")
	_ = d.Set("target_file", "docker-compose.yml")
	_ = d.Set("username", "user")
	_ = d.Set("password", "secret")

	if err := ds.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Id(); got != "gitops-repo-file-https://github.com/owner/repo.git-docker-compose.yml" {
		t.Errorf("unexpected ID: %q", got)
	}
	if got := d.Get("file_content"); got != "version: '3'\nservices:\n  app:\n    image: nginx\n" {
		t.Errorf("file_content mismatch: %v", got)
	}

	// Verify the POST payload — the data source uses camelCase keys and the
	// special-cased `TLSSkipVerify` (uppercase per server expectations).
	post := mock.FindRequest("POST", "/gitops/repo/file/preview")
	if post == nil {
		t.Fatal("expected POST /gitops/repo/file/preview to be sent")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload["repository"] != "https://github.com/owner/repo.git" {
		t.Errorf("payload.repository: got %v", payload["repository"])
	}
	if payload["reference"] != "refs/heads/main" {
		t.Errorf("payload.reference: got %v", payload["reference"])
	}
	if payload["targetFile"] != "docker-compose.yml" {
		t.Errorf("payload.targetFile: got %v", payload["targetFile"])
	}
	if payload["username"] != "user" {
		t.Errorf("payload.username: got %v", payload["username"])
	}
	if payload["password"] != "secret" {
		t.Errorf("payload.password: got %v", payload["password"])
	}
}

// TestDataSourceGitopsRepoFileRead_HTTPError verifies non-2xx is surfaced.
func TestDataSourceGitopsRepoFileRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/gitops/repo/file/preview", RespondString(
		http.StatusUnauthorized, "application/json",
		`{"message":"authentication required"}`,
	))

	ds := dataSourceGitopsRepoFile()
	d := ds.TestResourceData()
	_ = d.Set("repository_url", "https://example.com/repo.git")

	if err := ds.Read(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 401, got nil")
	}
}

// TestDataSourceGitopsRepoFileRead_BadJSON verifies a malformed response is
// surfaced rather than silently producing empty state.
func TestDataSourceGitopsRepoFileRead_BadJSON(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/gitops/repo/file/preview", RespondString(
		http.StatusOK, "application/json", `not-json-at-all`,
	))

	ds := dataSourceGitopsRepoFile()
	d := ds.TestResourceData()
	_ = d.Set("repository_url", "https://example.com/repo.git")

	if err := ds.Read(d, mock.Client()); err == nil {
		t.Fatal("expected decode error on malformed JSON, got nil")
	}
}
