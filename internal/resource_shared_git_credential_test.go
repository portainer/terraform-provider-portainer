package internal

import (
	"net/http"
	"testing"
)

// TestSharedGitCredentialCreate_UnwrapsResponse is a regression test for issue #120.
//
// Portainer's POST /cloud/gitcredentials returns the created entity wrapped in
// a "gitCredential" envelope:
//
//	{"gitCredential":{"id":42,"userId":0,"name":"...","username":"...","authorizationType":1}}
//
// Before the fix, the resource decoded directly into a flat {"id":int} struct,
// so result.ID was always 0. SetId("0") then triggered the follow-up Read at
// /cloud/gitcredentials/0, which Portainer rejected with HTTP 500
// "Object not found inside the database (bucket=git_credentials, key=0)".
//
// This test fails if the create response is ever decoded without unwrapping.
func TestSharedGitCredentialCreate_UnwrapsResponse(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/cloud/gitcredentials", RespondJSON(http.StatusOK, map[string]interface{}{
		"gitCredential": map[string]interface{}{
			"id":                42,
			"userId":            0,
			"name":              "github_test",
			"username":          "testtoken",
			"authorizationType": 1,
		},
	}))

	mock.On("GET", "/cloud/gitcredentials/42", RespondJSON(http.StatusOK, map[string]interface{}{
		"id":                42,
		"userId":            0,
		"name":              "github_test",
		"username":          "testtoken",
		"authorizationType": 1,
	}))

	r := resourcePortainerSharedGitCredential()
	d := r.TestResourceData()
	_ = d.Set("name", "github_test")
	_ = d.Set("username", "testtoken")
	_ = d.Set("password", "some_token")
	_ = d.Set("authorization_type", 1)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "42" {
		t.Fatalf("regression of issue #120: expected ID %q (from wrapped response), got %q — the create response envelope is not being unwrapped", "42", d.Id())
	}

	// Verify the POST payload uses the correct field names (camelCase, not capitalized).
	post := mock.FindRequest("POST", "/cloud/gitcredentials")
	if post == nil {
		t.Fatal("expected a POST to /cloud/gitcredentials")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode POST body: %v", err)
	}
	if got := payload["name"]; got != "github_test" {
		t.Errorf("payload.name: expected %q, got %v", "github_test", got)
	}
	if got := payload["username"]; got != "testtoken" {
		t.Errorf("payload.username: expected %q, got %v", "testtoken", got)
	}
	if got := payload["password"]; got != "some_token" {
		t.Errorf("payload.password: expected %q, got %v", "some_token", got)
	}
	// JSON numbers decode as float64 by default.
	if got := payload["authorizationType"]; got != float64(1) {
		t.Errorf("payload.authorizationType: expected 1, got %v", got)
	}

	// Verify Read was invoked with the correct ID and the resource state was populated.
	if mock.FindRequest("GET", "/cloud/gitcredentials/42") == nil {
		t.Error("expected Create to chain into Read at /cloud/gitcredentials/42")
	}
	if got := d.Get("user_id"); got != 0 {
		t.Errorf("user_id: expected 0, got %v", got)
	}
	if got := d.Get("authorization_type"); got != 1 {
		t.Errorf("authorization_type: expected 1, got %v", got)
	}
}

// TestSharedGitCredentialCreate_HTTPError verifies that an HTTP 4xx/5xx
// response is surfaced as an error rather than silently setting an empty ID.
func TestSharedGitCredentialCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/cloud/gitcredentials", RespondString(
		http.StatusBadRequest, "application/json",
		`{"message":"invalid authorization type"}`,
	))

	r := resourcePortainerSharedGitCredential()
	d := r.TestResourceData()
	_ = d.Set("name", "bad")
	_ = d.Set("username", "x")
	_ = d.Set("password", "y")
	_ = d.Set("authorization_type", 1)

	err := rcCreate(r, d, mock.Client())
	if err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}

// TestSharedGitCredentialRead_FlatResponse verifies that the Read endpoint
// (which returns a flat object, NOT wrapped — confirmed by the original issue
// and by parity with resource_user_git_credential.go) correctly populates state.
func TestSharedGitCredentialRead_FlatResponse(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/cloud/gitcredentials/7", RespondJSON(http.StatusOK, map[string]interface{}{
		"id":                7,
		"userId":            3,
		"name":              "ci_token",
		"username":          "robot",
		"authorizationType": 1,
	}))

	r := resourcePortainerSharedGitCredential()
	d := r.TestResourceData()
	d.SetId("7")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Get("name"); got != "ci_token" {
		t.Errorf("name: expected %q, got %v", "ci_token", got)
	}
	if got := d.Get("user_id"); got != 3 {
		t.Errorf("user_id: expected 3, got %v", got)
	}
	if got := d.Get("authorization_type"); got != 1 {
		t.Errorf("authorization_type: expected 1, got %v", got)
	}
}

// TestSharedGitCredentialRead_404_ClearsID verifies that a 404 on Read removes
// the resource from state (standard Terraform drift-detection pattern).
func TestSharedGitCredentialRead_404_ClearsID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/cloud/gitcredentials/99", RespondString(
		http.StatusNotFound, "application/json",
		`{"message":"not found"}`,
	))

	r := resourcePortainerSharedGitCredential()
	d := r.TestResourceData()
	d.SetId("99")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read should swallow 404 and clear ID, got error: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared after 404, got %q", d.Id())
	}
}
