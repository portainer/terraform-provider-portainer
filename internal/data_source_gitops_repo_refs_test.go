package internal

import (
	"net/http"
	"testing"
)

// TestDataSourceGitopsRepoRefsRead_HappyPath verifies the data source POSTs to
// /gitops/repo/refs and stores the returned ref list.
func TestDataSourceGitopsRepoRefsRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/gitops/repo/refs", RespondJSON(http.StatusOK, []string{
		"refs/heads/main",
		"refs/heads/develop",
		"refs/tags/v1.0.0",
	}))

	ds := dataSourceGitopsRepoRefs()
	d := ds.TestResourceData()
	_ = d.Set("repository_url", "https://github.com/owner/repo.git")

	if err := ds.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Id(); got != "gitops-repo-refs-https://github.com/owner/repo.git" {
		t.Errorf("unexpected ID: %q", got)
	}

	refs := d.Get("refs").([]interface{})
	if len(refs) != 3 {
		t.Fatalf("expected 3 refs, got %d", len(refs))
	}
	if refs[0] != "refs/heads/main" {
		t.Errorf("refs[0]: got %v", refs[0])
	}
	if refs[2] != "refs/tags/v1.0.0" {
		t.Errorf("refs[2]: got %v", refs[2])
	}
}

// TestDataSourceGitopsRepoRefsRead_PayloadShape verifies that auth fields, when
// present, are forwarded in the POST body with the expected JSON keys.
func TestDataSourceGitopsRepoRefsRead_PayloadShape(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/gitops/repo/refs", RespondJSON(http.StatusOK, []string{}))

	ds := dataSourceGitopsRepoRefs()
	d := ds.TestResourceData()
	_ = d.Set("repository_url", "https://example.com/repo.git")
	_ = d.Set("username", "robot")
	_ = d.Set("password", "secret")
	_ = d.Set("git_credential_id", 42)

	if err := ds.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	post := mock.FindRequest("POST", "/gitops/repo/refs")
	if post == nil {
		t.Fatal("expected POST /gitops/repo/refs to be sent")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if payload["repository"] != "https://example.com/repo.git" {
		t.Errorf("payload.repository: got %v", payload["repository"])
	}
	if payload["username"] != "robot" {
		t.Errorf("payload.username: got %v", payload["username"])
	}
	if payload["password"] != "secret" {
		t.Errorf("payload.password: got %v", payload["password"])
	}
	// JSON numbers decode as float64.
	if payload["gitCredentialID"] != float64(42) {
		t.Errorf("payload.gitCredentialID: got %v", payload["gitCredentialID"])
	}
}

// TestDataSourceGitopsRepoRefsRead_HTTPError verifies a 4xx response is
// surfaced as an error.
func TestDataSourceGitopsRepoRefsRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/gitops/repo/refs", RespondString(
		http.StatusBadRequest, "application/json",
		`{"message":"invalid repository URL"}`,
	))

	ds := dataSourceGitopsRepoRefs()
	d := ds.TestResourceData()
	_ = d.Set("repository_url", "not-a-url")

	if err := ds.Read(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
}
