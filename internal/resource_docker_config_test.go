package internal

import (
	"net/http"
	"testing"
)

// TestDockerConfigCreate_HappyPath verifies the resource lists configs to
// detect duplicates, POSTs the create body, and stores the returned ID and
// resource_control_id.
func TestDockerConfigCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	// Duplicate-detection GET returns an empty list.
	mock.On("GET", "/endpoints/1/docker/configs", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	mock.On("POST", "/endpoints/1/docker/configs/create", RespondJSON(http.StatusOK, map[string]interface{}{
		"ID": "abc123",
		"Portainer": map[string]interface{}{
			"ResourceControl": map[string]interface{}{"Id": 42},
		},
	}))

	r := resourceDockerConfig()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "my-config")
	_ = d.Set("data", "c2VjcmV0")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "abc123" {
		t.Errorf("expected ID %q, got %q", "abc123", d.Id())
	}
	if got := d.Get("resource_control_id"); got != 42 {
		t.Errorf("resource_control_id: expected 42, got %v", got)
	}

	post := mock.FindRequest("POST", "/endpoints/1/docker/configs/create")
	if post == nil {
		t.Fatal("expected a POST to /endpoints/1/docker/configs/create")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode POST body: %v", err)
	}
	if got := payload["Name"]; got != "my-config" {
		t.Errorf("payload.Name: expected %q, got %v", "my-config", got)
	}
	if got := payload["Data"]; got != "c2VjcmV0" {
		t.Errorf("payload.Data: expected %q, got %v", "c2VjcmV0", got)
	}
}

// TestDockerConfigRead_HappyPath verifies Read populates name and labels from
// the inspect endpoint payload.
func TestDockerConfigRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/configs/abc", RespondJSON(http.StatusOK, map[string]interface{}{
		"ID": "abc",
		"Spec": map[string]interface{}{
			"Name": "my-config",
			"Labels": map[string]interface{}{
				"env": "prod",
			},
		},
		"Portainer": map[string]interface{}{
			"ResourceControl": map[string]interface{}{"Id": 7},
		},
	}))

	r := resourceDockerConfig()
	d := r.TestResourceData()
	d.SetId("abc")
	_ = d.Set("endpoint_id", 1)

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Get("name"); got != "my-config" {
		t.Errorf("name: expected %q, got %v", "my-config", got)
	}
	if got := d.Get("resource_control_id"); got != 7 {
		t.Errorf("resource_control_id: expected 7, got %v", got)
	}
}

// TestDockerConfigRead_404ClearsID verifies that a 404 on Read clears the ID.
func TestDockerConfigRead_404ClearsID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/configs/nope", RespondString(
		http.StatusNotFound, "application/json",
		`{"message":"not found"}`,
	))

	r := resourceDockerConfig()
	d := r.TestResourceData()
	d.SetId("nope")
	_ = d.Set("endpoint_id", 1)

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read should swallow 404, got error: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestDockerConfigUpdate_HappyPath verifies the update POST is sent and
// the chained Read repopulates state.
func TestDockerConfigUpdate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/docker/configs/abc/update", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/endpoints/1/docker/configs/abc", RespondJSON(http.StatusOK, map[string]interface{}{
		"ID": "abc",
		"Spec": map[string]interface{}{
			"Name":   "renamed",
			"Labels": map[string]interface{}{},
		},
	}))

	r := resourceDockerConfig()
	d := r.TestResourceData()
	d.SetId("abc")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "renamed")
	_ = d.Set("data", "newdata")

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	if mock.FindRequest("POST", "/endpoints/1/docker/configs/abc/update") == nil {
		t.Error("expected POST to /endpoints/1/docker/configs/abc/update")
	}
	if got := d.Get("name"); got != "renamed" {
		t.Errorf("name: expected %q, got %v", "renamed", got)
	}
}

// TestDockerConfigDelete_HappyPath verifies DELETE is sent and ID is cleared.
func TestDockerConfigDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/docker/configs/abc", RespondString(http.StatusNoContent, "", ""))

	r := resourceDockerConfig()
	d := r.TestResourceData()
	d.SetId("abc")
	_ = d.Set("endpoint_id", 1)

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if mock.FindRequest("DELETE", "/endpoints/1/docker/configs/abc") == nil {
		t.Error("expected DELETE /endpoints/1/docker/configs/abc")
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared after delete, got %q", d.Id())
	}
}

// TestDockerConfigCreate_HTTPError verifies that a 5xx response on create
// is surfaced as an error.
func TestDockerConfigCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/configs", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/endpoints/1/docker/configs/create", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"boom"}`,
	))

	r := resourceDockerConfig()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "x")
	_ = d.Set("data", "y")

	err := rcCreate(r, d, mock.Client())
	if err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}
