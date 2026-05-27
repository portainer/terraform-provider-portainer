package internal

import (
	"net/http"
	"testing"
)

// TestAuthCreate_HappyPath verifies that resourceAuth posts username/password
// to /auth, stores the returned JWT into the "jwt" attribute, and sets the
// resource ID to the sentinel "auth-result".
func TestAuthCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/auth", RespondJSON(http.StatusOK, map[string]string{
		"jwt": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.payload.sig",
	}))

	r := resourceAuth()
	d := r.TestResourceData()
	_ = d.Set("username", "admin")
	_ = d.Set("password", "secret")

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "auth-result" {
		t.Errorf("expected ID %q, got %q", "auth-result", d.Id())
	}
	if got := d.Get("jwt"); got != "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.payload.sig" {
		t.Errorf("jwt: got %v", got)
	}

	// Verify the request payload.
	req := mock.FindRequest("POST", "/auth")
	if req == nil {
		t.Fatal("expected POST /auth")
	}
	var payload map[string]interface{}
	if err := req.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if got := payload["username"]; got != "admin" {
		t.Errorf("payload.username: got %v", got)
	}
	if got := payload["password"]; got != "secret" {
		t.Errorf("payload.password: got %v", got)
	}
}

// TestAuthCreate_HTTPError verifies that a non-2xx response is surfaced as
// an error and the resource ID stays empty.
func TestAuthCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/auth", RespondString(
		http.StatusUnauthorized, "application/json",
		`{"message":"invalid credentials"}`,
	))

	r := resourceAuth()
	d := r.TestResourceData()
	_ = d.Set("username", "admin")
	_ = d.Set("password", "wrong")

	err := r.Create(d, mock.Client())
	if err == nil {
		t.Fatal("expected error on 401, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}
