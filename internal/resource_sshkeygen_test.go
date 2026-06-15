package internal

import (
	"net/http"
	"testing"
)

// TestSSHKeygenCreate_HappyPath verifies that the action POSTs to /sshkeygen,
// stores the returned public/private keys, and sets an ID derived from the
// public-key length.
func TestSSHKeygenCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	publicKey := "ssh-rsa AAAAB3NzaC1yc2EAAAAD test@host"
	privateKey := "-----BEGIN RSA PRIVATE KEY-----\nfake\n-----END RSA PRIVATE KEY-----"

	mock.On("POST", "/sshkeygen", RespondJSON(http.StatusOK, map[string]string{
		"public":  publicKey,
		"private": privateKey,
	}))

	r := resourcePortainerSSHKeygen()
	d := r.TestResourceData()

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() == "" {
		t.Error("expected non-empty ID after sshkeygen")
	}
	if got := d.Get("public"); got != publicKey {
		t.Errorf("public: got %v, want %v", got, publicKey)
	}
	if got := d.Get("private"); got != privateKey {
		t.Errorf("private: got %v, want %v", got, privateKey)
	}

	// Verify the request was sent with API key auth header.
	req := mock.FindRequest("POST", "/sshkeygen")
	if req == nil {
		t.Fatal("expected POST /sshkeygen")
	}
	if req.Headers.Get("X-API-Key") == "" {
		t.Error("expected X-API-Key header on sshkeygen request")
	}
}

// TestSSHKeygenCreate_HTTPError verifies that a server error is propagated.
func TestSSHKeygenCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/sshkeygen", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"keygen failed"}`,
	))

	r := resourcePortainerSSHKeygen()
	d := r.TestResourceData()

	err := rcCreate(r, d, mock.Client())
	if err == nil {
		t.Fatal("expected error on 500, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}
