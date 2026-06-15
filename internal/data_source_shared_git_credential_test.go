package internal

import (
	"net/http"
	"testing"
)

// TestDataSourceSharedGitCredentialRead_HappyPath verifies the DS lists
// /cloud/gitcredentials, filters by name, and populates the computed fields.
func TestDataSourceSharedGitCredentialRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/cloud/gitcredentials", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"id":                1,
			"userId":            10,
			"name":              "ci_token",
			"username":          "robot",
			"authorizationType": 1,
		},
		{
			"id":                2,
			"userId":            20,
			"name":              "ops_token",
			"username":          "ops",
			"authorizationType": 0,
		},
	}))

	ds := dataSourcePortainerSharedGitCredential()
	d := ds.TestResourceData()
	_ = d.Set("name", "ops_token")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "2" {
		t.Errorf("expected ID %q, got %q", "2", d.Id())
	}
	if got := d.Get("username"); got != "ops" {
		t.Errorf("username: expected %q, got %v", "ops", got)
	}
	if got := d.Get("authorization_type"); got != 0 {
		t.Errorf("authorization_type: expected 0, got %v", got)
	}
	if got := d.Get("user_id"); got != 20 {
		t.Errorf("user_id: expected 20, got %v", got)
	}
}

// TestDataSourceSharedGitCredentialRead_NotFound verifies the error path.
func TestDataSourceSharedGitCredentialRead_NotFound(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/cloud/gitcredentials", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"id": 1, "userId": 10, "name": "ci_token", "username": "robot", "authorizationType": 1},
	}))

	ds := dataSourcePortainerSharedGitCredential()
	d := ds.TestResourceData()
	_ = d.Set("name", "ghost")

	if err := rcRead(ds, d, mock.Client()); err == nil {
		t.Fatal("expected error when credential not found, got nil")
	}
}

// TestDataSourceSharedGitCredentialRead_HTTPError verifies HTTP errors
// propagate.
func TestDataSourceSharedGitCredentialRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/cloud/gitcredentials", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"boom"}`,
	))

	ds := dataSourcePortainerSharedGitCredential()
	d := ds.TestResourceData()
	_ = d.Set("name", "ci_token")

	if err := rcRead(ds, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}
