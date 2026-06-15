package internal

import (
	"net/http"
	"testing"
)

// TestSharedGitCredentialUpdate_HappyPath covers the PUT update path and the
// chained Read.
func TestSharedGitCredentialUpdate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/cloud/gitcredentials/5", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/cloud/gitcredentials/5", RespondJSON(http.StatusOK, map[string]interface{}{
		"id":                5,
		"userId":            2,
		"name":              "updated",
		"username":          "robot",
		"authorizationType": 1,
	}))

	r := resourcePortainerSharedGitCredential()
	d := r.TestResourceData()
	d.SetId("5")
	_ = d.Set("name", "updated")
	_ = d.Set("username", "robot")
	_ = d.Set("password", "secret")
	_ = d.Set("authorization_type", 1)

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/cloud/gitcredentials/5")
	if put == nil {
		t.Fatal("expected PUT /cloud/gitcredentials/5")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["name"] != "updated" {
		t.Errorf("payload.name: expected %q, got %v", "updated", payload["name"])
	}
	if got := d.Get("name"); got != "updated" {
		t.Errorf("name: expected %q, got %v", "updated", got)
	}
}

// TestSharedGitCredentialUpdate_HTTPError covers the PUT >= 400 branch.
func TestSharedGitCredentialUpdate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/cloud/gitcredentials/5", RespondString(
		http.StatusBadRequest, "application/json", `{"message":"nope"}`))

	r := resourcePortainerSharedGitCredential()
	d := r.TestResourceData()
	d.SetId("5")
	_ = d.Set("name", "x")
	_ = d.Set("username", "y")
	_ = d.Set("password", "z")

	if err := rcUpdate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on PUT 400, got nil")
	}
}

// TestSharedGitCredentialRead_HTTPError covers a non-404 error on Read.
func TestSharedGitCredentialRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/cloud/gitcredentials/5", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourcePortainerSharedGitCredential()
	d := r.TestResourceData()
	d.SetId("5")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on Read 500, got nil")
	}
}

// TestSharedGitCredentialDelete_HappyPath covers the DELETE success path.
func TestSharedGitCredentialDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/cloud/gitcredentials/8", RespondString(http.StatusNoContent, "", ""))

	r := resourcePortainerSharedGitCredential()
	d := r.TestResourceData()
	d.SetId("8")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/cloud/gitcredentials/8") == nil {
		t.Error("expected DELETE /cloud/gitcredentials/8")
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared after delete, got %q", d.Id())
	}
}

// TestSharedGitCredentialDelete_404IsSuccess covers the 404-tolerant delete branch.
func TestSharedGitCredentialDelete_404IsSuccess(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/cloud/gitcredentials/9", RespondString(
		http.StatusNotFound, "application/json", `{"message":"gone"}`))

	r := resourcePortainerSharedGitCredential()
	d := r.TestResourceData()
	d.SetId("9")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete should treat 404 as success, got: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestSharedGitCredentialDelete_HTTPError covers a non-404 error on delete.
func TestSharedGitCredentialDelete_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/cloud/gitcredentials/9", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourcePortainerSharedGitCredential()
	d := r.TestResourceData()
	d.SetId("9")

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on DELETE 500, got nil")
	}
}
