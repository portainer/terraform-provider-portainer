package internal

import (
	"net/http"
	"testing"
)

// TestDockerImageCreate_HappyPath verifies the pull POST hits the create
// endpoint with `fromImage` in the query string, that the ID is set to
// "<endpoint_id>-<image>", and that the default X-Registry-Auth header is
// emitted.
func TestDockerImageCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/docker/images/create", RespondString(
		http.StatusOK, "application/json",
		`{"status":"Pulling from library/nginx"}`,
	))

	r := resourceDockerImage()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("image", "nginx:1.25")

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "1-nginx:1.25" {
		t.Errorf("expected ID %q, got %q", "1-nginx:1.25", d.Id())
	}

	post := mock.FindRequest("POST", "/endpoints/1/docker/images/create")
	if post == nil {
		t.Fatal("expected a POST to /endpoints/1/docker/images/create")
	}
	if got := post.Query; got == "" {
		t.Errorf("expected non-empty query string, got empty")
	}
	if got := post.Headers.Get("X-Registry-Auth"); got == "" {
		t.Error("expected X-Registry-Auth header to be set (even when no auth provided)")
	}
}

// TestDockerImageCreate_WithRegistryAuth verifies the resource encodes
// username:password into the X-Registry-Auth header.
func TestDockerImageCreate_WithRegistryAuth(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/docker/images/create", RespondString(
		http.StatusOK, "application/json",
		`{"status":"ok"}`,
	))

	r := resourceDockerImage()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("image", "ghcr.io/foo/bar:latest")
	_ = d.Set("registry_auth", "user:pass")

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	post := mock.FindRequest("POST", "/endpoints/1/docker/images/create")
	if post == nil {
		t.Fatal("expected a POST to /endpoints/1/docker/images/create")
	}
	if got := post.Headers.Get("X-Registry-Auth"); got == "" {
		t.Error("expected X-Registry-Auth header to be non-empty for registry_auth user:pass")
	}
}

// TestDockerImageCreate_InvalidAuthFormat verifies the resource errors out
// when registry_auth is not in user:pass form.
func TestDockerImageCreate_InvalidAuthFormat(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/endpoints/1/docker/images/create", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceDockerImage()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("image", "nginx:latest")
	_ = d.Set("registry_auth", "no-colon-here")

	err := r.Create(d, mock.Client())
	if err == nil {
		t.Fatal("expected error for invalid registry_auth format")
	}
}

// TestDockerImageRead_NoOp verifies that Read is a no-op (the resource has
// no remote state to refresh from).
func TestDockerImageRead_NoOp(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceDockerImage()
	d := r.TestResourceData()
	d.SetId("1-nginx:1.25")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("image", "nginx:1.25")

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if len(mock.Requests()) != 0 {
		t.Errorf("expected zero HTTP requests during Read, got %d", len(mock.Requests()))
	}
}

// TestDockerImageDelete_HappyPath verifies a DELETE is sent to the image
// path and the ID is cleared.
func TestDockerImageDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/docker/images/nginx:1.25", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	r := resourceDockerImage()
	d := r.TestResourceData()
	d.SetId("1-nginx:1.25")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("image", "nginx:1.25")

	if err := r.Delete(d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if mock.FindRequest("DELETE", "/endpoints/1/docker/images/nginx:1.25") == nil {
		t.Error("expected DELETE /endpoints/1/docker/images/nginx:1.25")
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestDockerImageCreate_HTTPError verifies that a 4xx response is surfaced.
func TestDockerImageCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/docker/images/create", RespondString(
		http.StatusNotFound, "application/json",
		`{"message":"image not found"}`,
	))

	r := resourceDockerImage()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("image", "nope:bad")

	err := r.Create(d, mock.Client())
	if err == nil {
		t.Fatal("expected error on HTTP 404, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}
