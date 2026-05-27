package internal

import (
	"net/http"
	"testing"
)

// TestUserAdminCreate_HappyPath verifies that POST /users/admin/init is sent
// with the username/password and that the resource captures the returned ID.
func TestUserAdminCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/users/admin/init", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":       1,
		"Username": "admin",
		"Role":     1,
	}))

	r := resourceUserAdmin()
	d := r.TestResourceData()
	_ = d.Set("username", "admin")
	_ = d.Set("password", "S3cret!password")

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "1" {
		t.Errorf("expected ID %q, got %q", "1", d.Id())
	}
	if got := d.Get("initialized"); got != true {
		t.Errorf("initialized: expected true, got %v", got)
	}

	post := mock.FindRequest("POST", "/users/admin/init")
	if post == nil {
		t.Fatal("expected POST /users/admin/init")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode POST: %v", err)
	}
	if got := payload["username"]; got != "admin" {
		t.Errorf("payload.username: expected %q, got %v", "admin", got)
	}
	if got := payload["password"]; got != "S3cret!password" {
		t.Errorf("payload.password: expected %q, got %v", "S3cret!password", got)
	}
}

// TestUserAdminCreate_Conflict_TreatedAsIdempotent verifies that a 409
// response (admin already initialised) is swallowed and the resource still
// ends up with a sentinel ID + initialized=true.
func TestUserAdminCreate_Conflict_TreatedAsIdempotent(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/users/admin/init", RespondString(
		http.StatusConflict, "application/json",
		`{"message":"admin already initialized"}`,
	))

	r := resourceUserAdmin()
	d := r.TestResourceData()
	_ = d.Set("username", "admin")
	_ = d.Set("password", "anything")

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("409 should be treated as idempotent success, got: %v", err)
	}
	if d.Id() != "portainer-admin" {
		t.Errorf("expected fallback ID %q, got %q", "portainer-admin", d.Id())
	}
	if got := d.Get("initialized"); got != true {
		t.Errorf("initialized: expected true after 409, got %v", got)
	}
}

// TestUserAdminCreate_ZeroID_FallsBack verifies that when the API returns a
// successful payload but without an Id, the resource falls back to a stable
// sentinel ID.
func TestUserAdminCreate_ZeroID_FallsBack(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/users/admin/init", RespondJSON(http.StatusOK, map[string]interface{}{
		"Username": "admin",
		// no Id field
	}))

	r := resourceUserAdmin()
	d := r.TestResourceData()
	_ = d.Set("username", "admin")
	_ = d.Set("password", "pw")

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "portainer-admin" {
		t.Errorf("expected fallback ID %q, got %q", "portainer-admin", d.Id())
	}
}

// TestUserAdminCreate_HTTPError verifies that a non-409 server error is
// propagated.
func TestUserAdminCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/users/admin/init", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"boom"}`,
	))

	r := resourceUserAdmin()
	d := r.TestResourceData()
	_ = d.Set("username", "admin")
	_ = d.Set("password", "pw")

	if err := r.Create(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestUserAdminRead_AlwaysClearsID verifies the documented Read behaviour —
// this resource is bootstrap-only and Read/Update/Delete just clear the ID.
func TestUserAdminRead_AlwaysClearsID(t *testing.T) {
	mock := NewMockServer(t)
	r := resourceUserAdmin()
	d := r.TestResourceData()
	d.SetId("1")

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared by Read, got %q", d.Id())
	}
}
