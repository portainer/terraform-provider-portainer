package internal

import (
	"net/http"
	"testing"
)

// TestUserGitCredentialCreate_HappyPath verifies the POST is sent, the
// composite ID is set, and the chained Read repopulates state.
func TestUserGitCredentialCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/users/3/gitcredentials", RespondJSON(http.StatusOK, map[string]interface{}{
		"gitCredential": map[string]interface{}{"id": 9},
	}))
	mock.On("GET", "/users/3/gitcredentials/9", RespondJSON(http.StatusOK, map[string]interface{}{
		"id":                9,
		"name":              "my-cred",
		"username":          "git-user",
		"authorizationType": 1,
		"userId":            3,
	}))

	r := resourcePortainerUserGitCredential()
	d := r.TestResourceData()
	_ = d.Set("user_id", 3)
	_ = d.Set("name", "my-cred")
	_ = d.Set("username", "git-user")
	_ = d.Set("password", "tok")
	_ = d.Set("authorization_type", 1)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "3:9" {
		t.Errorf("expected ID %q, got %q", "3:9", d.Id())
	}
	if got := d.Get("credential_id"); got != 9 {
		t.Errorf("credential_id: expected 9, got %v", got)
	}
	if got := d.Get("username"); got != "git-user" {
		t.Errorf("username: expected %q, got %v", "git-user", got)
	}
	if got := d.Get("authorization_type"); got != 1 {
		t.Errorf("authorization_type: expected 1, got %v", got)
	}

	post := mock.FindRequest("POST", "/users/3/gitcredentials")
	if post == nil {
		t.Fatal("expected POST to /users/3/gitcredentials")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if got := payload["name"]; got != "my-cred" {
		t.Errorf("payload.name: expected %q, got %v", "my-cred", got)
	}
	if got := payload["authorizationType"]; got != float64(1) {
		t.Errorf("payload.authorizationType: expected 1, got %v", got)
	}
}

// TestUserGitCredentialCreate_HTTPError verifies a non-2xx create surfaces an error.
func TestUserGitCredentialCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/users/3/gitcredentials", RespondString(
		http.StatusBadRequest, "application/json", `{"message":"invalid"}`,
	))

	r := resourcePortainerUserGitCredential()
	d := r.TestResourceData()
	_ = d.Set("user_id", 3)
	_ = d.Set("name", "x")
	_ = d.Set("username", "u")
	_ = d.Set("password", "p")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
}

// TestUserGitCredentialRead_HappyPath verifies Read populates state.
func TestUserGitCredentialRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/users/3/gitcredentials/9", RespondJSON(http.StatusOK, map[string]interface{}{
		"id":                9,
		"name":              "my-cred",
		"username":          "git-user",
		"authorizationType": 0,
		"userId":            3,
	}))

	r := resourcePortainerUserGitCredential()
	d := r.TestResourceData()
	d.SetId("3:9")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Get("user_id"); got != 3 {
		t.Errorf("user_id: expected 3, got %v", got)
	}
	if got := d.Get("name"); got != "my-cred" {
		t.Errorf("name: expected %q, got %v", "my-cred", got)
	}
}

// TestUserGitCredentialRead_404ClearsID verifies a 404 clears the ID.
func TestUserGitCredentialRead_404ClearsID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/users/3/gitcredentials/9", RespondString(
		http.StatusNotFound, "application/json", `{"message":"not found"}`,
	))

	r := resourcePortainerUserGitCredential()
	d := r.TestResourceData()
	d.SetId("3:9")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read should swallow 404, got error: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestUserGitCredentialRead_BadID verifies a malformed ID surfaces an error.
func TestUserGitCredentialRead_BadID(t *testing.T) {
	mock := NewMockServer(t)

	r := resourcePortainerUserGitCredential()
	d := r.TestResourceData()
	d.SetId("not-a-valid-id")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for malformed ID, got nil")
	}
}

// TestUserGitCredentialUpdate_HappyPath verifies the PUT is sent and chained Read runs.
func TestUserGitCredentialUpdate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/users/3/gitcredentials/9", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/users/3/gitcredentials/9", RespondJSON(http.StatusOK, map[string]interface{}{
		"id":                9,
		"name":              "renamed",
		"username":          "git-user",
		"authorizationType": 0,
		"userId":            3,
	}))

	r := resourcePortainerUserGitCredential()
	d := r.TestResourceData()
	d.SetId("3:9")
	_ = d.Set("name", "renamed")
	_ = d.Set("username", "git-user")
	_ = d.Set("password", "p")

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if mock.FindRequest("PUT", "/users/3/gitcredentials/9") == nil {
		t.Error("expected PUT to gitcredentials endpoint")
	}
	if got := d.Get("name"); got != "renamed" {
		t.Errorf("name: expected %q, got %v", "renamed", got)
	}
}

// TestUserGitCredentialUpdate_HTTPError verifies a non-2xx update surfaces an error.
func TestUserGitCredentialUpdate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/users/3/gitcredentials/9", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourcePortainerUserGitCredential()
	d := r.TestResourceData()
	d.SetId("3:9")
	_ = d.Set("name", "x")
	_ = d.Set("username", "u")
	_ = d.Set("password", "p")

	if err := rcUpdate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestUserGitCredentialDelete_HappyPath verifies DELETE is sent and ID cleared.
func TestUserGitCredentialDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/users/3/gitcredentials/9", RespondString(http.StatusNoContent, "", ""))

	r := resourcePortainerUserGitCredential()
	d := r.TestResourceData()
	d.SetId("3:9")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/users/3/gitcredentials/9") == nil {
		t.Error("expected DELETE to gitcredentials endpoint")
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestUserGitCredentialDelete_404IsSuccess verifies a 404 on delete is treated as success.
func TestUserGitCredentialDelete_404IsSuccess(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/users/3/gitcredentials/9", RespondString(
		http.StatusNotFound, "application/json", `{"message":"gone"}`,
	))

	r := resourcePortainerUserGitCredential()
	d := r.TestResourceData()
	d.SetId("3:9")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete should swallow 404, got error: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestUserGitCredentialDelete_HTTPError verifies a non-2xx/non-404 delete surfaces an error.
func TestUserGitCredentialDelete_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/users/3/gitcredentials/9", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourcePortainerUserGitCredential()
	d := r.TestResourceData()
	d.SetId("3:9")

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}
