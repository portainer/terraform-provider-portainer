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

	if err := rcCreate(r, d, mock.Client()); err != nil {
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

	if err := rcCreate(r, d, mock.Client()); err != nil {
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

	if err := rcCreate(r, d, mock.Client()); err != nil {
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

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestUserAdminRead_KeepsInitializedInstance is the regression test for the
// never-converging plan: Read used to clear the ID unconditionally, so every
// plan proposed the bootstrap again.
func TestUserAdminRead_KeepsInitializedInstance(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/users/admin/check", RespondString(http.StatusNoContent, "", ""))
	r := resourceUserAdmin()
	d := r.TestResourceData()
	d.SetId("1")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "1" {
		t.Errorf("expected ID kept for an initialized instance, got %q", d.Id())
	}
	if !d.Get("initialized").(bool) {
		t.Error("expected initialized = true")
	}
}

// TestUserAdminRead_DropsWhenAdminMissing covers a reset instance: without an
// administrator the bootstrap has to be planned again.
func TestUserAdminRead_DropsWhenAdminMissing(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/users/admin/check", RespondJSON(http.StatusNotFound, map[string]string{"message": "No administrator account found inside the database"}))
	r := resourceUserAdmin()
	d := r.TestResourceData()
	d.SetId("1")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared when no admin exists, got %q", d.Id())
	}
}

func TestUserAdminRead_ReportsServerError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/users/admin/check", RespondJSON(http.StatusInternalServerError, map[string]string{"message": "boom"}))
	r := resourceUserAdmin()
	d := r.TestResourceData()
	d.SetId("1")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected an error on HTTP 500")
	}
}

// TestUserAdminCreate_SendsSetupToken covers Portainer 2.45+, which refuses
// to initialize a fresh instance without the X-Setup-Token header.
func TestUserAdminCreate_SendsSetupToken(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/users/admin/init", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 1, "Username": "admin"}))

	r := resourceUserAdmin()
	d := r.TestResourceData()
	_ = d.Set("username", "admin")
	_ = d.Set("password", "pw-123456789")
	_ = d.Set("setup_token", "tok-abc")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	req := mock.FindRequest("POST", "/users/admin/init")
	if req == nil {
		t.Fatal("expected POST /users/admin/init")
	}
	if got := req.Headers.Get("X-Setup-Token"); got != "tok-abc" {
		t.Errorf("X-Setup-Token: expected %q, got %q", "tok-abc", got)
	}
	var body map[string]interface{}
	if err := req.DecodeJSON(&body); err != nil || body["username"] != "admin" {
		t.Errorf("expected the init payload to be sent unchanged, got %v (%v)", body, err)
	}
}

// TestUserAdminCreate_SkipsInitWhenAdminExists covers an initialized
// Portainer 2.45, which answers the init call with 403 (setup token) rather
// than 409 until it restarts: the existing admin is detected up front.
func TestUserAdminCreate_SkipsInitWhenAdminExists(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/users/admin/check", RespondString(http.StatusNoContent, "", ""))

	r := resourceUserAdmin()
	d := r.TestResourceData()
	_ = d.Set("username", "admin")
	_ = d.Set("password", "pw-123456789")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if mock.FindRequest("POST", "/users/admin/init") != nil {
		t.Error("expected no init call on an instance that already has an admin")
	}
	if d.Id() != "portainer-admin" || !d.Get("initialized").(bool) {
		t.Errorf("expected the resource marked initialized, got ID %q initialized=%v", d.Id(), d.Get("initialized"))
	}
}
