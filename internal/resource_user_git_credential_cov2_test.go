package internal

import (
	"context"
	"net/http"
	"testing"
)

// =========================================================================
// Additional coverage (cov2) for resource_user_git_credential.go: the
// Importer state func (happy + malformed), the Update/Delete malformed-ID
// guards, and the create-response decode-error branch.
// =========================================================================

// TestUserGitCredentialCov2_Import_HappyPath exercises the Importer state func
// with a "<user_id>:<credential_id>" ID; it should set user_id and keep the
// composite ID intact.
func TestUserGitCredentialCov2_Import_HappyPath(t *testing.T) {
	r := resourcePortainerUserGitCredential()
	d := r.TestResourceData()
	d.SetId("3:9")

	out, err := r.Importer.StateContext(context.Background(), d, nil)
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 resource data, got %d", len(out))
	}
	rd := out[0]
	if rd.Id() != "3:9" {
		t.Errorf("expected ID 3:9, got %q", rd.Id())
	}
	if got := rd.Get("user_id"); got != 3 {
		t.Errorf("user_id: expected 3, got %v", got)
	}
}

// TestUserGitCredentialCov2_Import_BadID verifies the import guard rejects a
// malformed composite ID.
func TestUserGitCredentialCov2_Import_BadID(t *testing.T) {
	r := resourcePortainerUserGitCredential()
	d := r.TestResourceData()
	d.SetId("not-valid")

	if _, err := r.Importer.StateContext(context.Background(), d, nil); err == nil {
		t.Fatal("expected error for malformed import ID, got nil")
	}
}

// TestUserGitCredentialCov2_Import_NonNumericUser verifies the non-numeric
// user-id branch of the importer.
func TestUserGitCredentialCov2_Import_NonNumericUser(t *testing.T) {
	r := resourcePortainerUserGitCredential()
	d := r.TestResourceData()
	d.SetId("abc:9")

	if _, err := r.Importer.StateContext(context.Background(), d, nil); err == nil {
		t.Fatal("expected error for non-numeric user ID in import, got nil")
	}
}

// TestUserGitCredentialCov2_Import_NonNumericCredential verifies the
// non-numeric credential-id branch of the importer.
func TestUserGitCredentialCov2_Import_NonNumericCredential(t *testing.T) {
	r := resourcePortainerUserGitCredential()
	d := r.TestResourceData()
	d.SetId("3:abc")

	if _, err := r.Importer.StateContext(context.Background(), d, nil); err == nil {
		t.Fatal("expected error for non-numeric credential ID in import, got nil")
	}
}

// TestUserGitCredentialCov2_Update_BadID verifies Update surfaces an error when
// the resource ID is malformed (parseUserGitCredentialID fails before any
// request).
func TestUserGitCredentialCov2_Update_BadID(t *testing.T) {
	mock := NewMockServer(t)

	r := resourcePortainerUserGitCredential()
	d := r.TestResourceData()
	d.SetId("garbage")
	_ = d.Set("name", "x")
	_ = d.Set("username", "u")
	_ = d.Set("password", "p")

	if err := rcUpdate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for malformed ID on Update, got nil")
	}
	if len(mock.Requests()) != 0 {
		t.Errorf("expected no HTTP requests on bad-ID Update, got %d", len(mock.Requests()))
	}
}

// TestUserGitCredentialCov2_Delete_BadID verifies Delete surfaces an error when
// the resource ID is malformed.
func TestUserGitCredentialCov2_Delete_BadID(t *testing.T) {
	mock := NewMockServer(t)

	r := resourcePortainerUserGitCredential()
	d := r.TestResourceData()
	d.SetId("garbage")

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for malformed ID on Delete, got nil")
	}
	if len(mock.Requests()) != 0 {
		t.Errorf("expected no HTTP requests on bad-ID Delete, got %d", len(mock.Requests()))
	}
}

// TestUserGitCredentialCov2_Create_DecodeError verifies the create-response
// decode-error branch: a 200 with a non-JSON body fails decoding.
func TestUserGitCredentialCov2_Create_DecodeError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/users/3/gitcredentials", RespondString(
		http.StatusOK, "application/json", `not-json`,
	))

	r := resourcePortainerUserGitCredential()
	d := r.TestResourceData()
	_ = d.Set("user_id", 3)
	_ = d.Set("name", "x")
	_ = d.Set("username", "u")
	_ = d.Set("password", "p")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected decode error on malformed create response, got nil")
	}
}
