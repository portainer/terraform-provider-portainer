package internal

import (
	"net/http"
	"testing"
)

// TestDockerSecretCreate_HappyPath verifies the resource lists secrets for
// dupes, POSTs the create body, and stores the returned ID and
// resource_control_id.
func TestDockerSecretCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/secrets", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	mock.On("POST", "/endpoints/1/docker/secrets/create", RespondJSON(http.StatusOK, map[string]interface{}{
		"ID": "sec_123",
		"Portainer": map[string]interface{}{
			"ResourceControl": map[string]interface{}{"Id": 11},
		},
	}))

	r := resourceDockerSecret()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "db-password")
	_ = d.Set("data", "c2VjcmV0")

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "sec_123" {
		t.Errorf("expected ID %q, got %q", "sec_123", d.Id())
	}
	if got := d.Get("resource_control_id"); got != 11 {
		t.Errorf("resource_control_id: expected 11, got %v", got)
	}

	post := mock.FindRequest("POST", "/endpoints/1/docker/secrets/create")
	if post == nil {
		t.Fatal("expected a POST to /endpoints/1/docker/secrets/create")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode POST body: %v", err)
	}
	if got := payload["Name"]; got != "db-password" {
		t.Errorf("payload.Name: expected %q, got %v", "db-password", got)
	}
	if got := payload["Data"]; got != "c2VjcmV0" {
		t.Errorf("payload.Data: expected %q, got %v", "c2VjcmV0", got)
	}
}

// TestDockerSecretRead_HappyPath verifies Read populates name, labels and
// resource_control_id from the inspect endpoint payload.
func TestDockerSecretRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/secrets/sec_1", RespondJSON(http.StatusOK, map[string]interface{}{
		"ID": "sec_1",
		"Spec": map[string]interface{}{
			"Name": "db-password",
			"Labels": map[string]interface{}{
				"env": "prod",
			},
		},
		"Portainer": map[string]interface{}{
			"ResourceControl": map[string]interface{}{"Id": 5},
		},
	}))

	r := resourceDockerSecret()
	d := r.TestResourceData()
	d.SetId("sec_1")
	_ = d.Set("endpoint_id", 1)

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Get("name"); got != "db-password" {
		t.Errorf("name: expected %q, got %v", "db-password", got)
	}
	if got := d.Get("resource_control_id"); got != 5 {
		t.Errorf("resource_control_id: expected 5, got %v", got)
	}
}

// TestDockerSecretRead_404ClearsID verifies that a 404 on Read clears the ID.
func TestDockerSecretRead_404ClearsID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/secrets/nope", RespondString(
		http.StatusNotFound, "application/json",
		`{"message":"not found"}`,
	))

	r := resourceDockerSecret()
	d := r.TestResourceData()
	d.SetId("nope")
	_ = d.Set("endpoint_id", 1)

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read should swallow 404, got error: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestDockerSecretUpdate_HappyPath verifies the update POST is sent and the
// chained Read repopulates state.
func TestDockerSecretUpdate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/docker/secrets/sec_1/update", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/endpoints/1/docker/secrets/sec_1", RespondJSON(http.StatusOK, map[string]interface{}{
		"ID": "sec_1",
		"Spec": map[string]interface{}{
			"Name":   "renamed",
			"Labels": map[string]interface{}{},
		},
	}))

	r := resourceDockerSecret()
	d := r.TestResourceData()
	d.SetId("sec_1")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "renamed")
	_ = d.Set("data", "newdata")

	if err := r.Update(d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	if mock.FindRequest("POST", "/endpoints/1/docker/secrets/sec_1/update") == nil {
		t.Error("expected POST /endpoints/1/docker/secrets/sec_1/update")
	}
	if got := d.Get("name"); got != "renamed" {
		t.Errorf("name: expected %q, got %v", "renamed", got)
	}
}

// TestDockerSecretDelete_HappyPath verifies DELETE is sent and ID is cleared.
func TestDockerSecretDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/docker/secrets/sec_1", RespondString(http.StatusNoContent, "", ""))

	r := resourceDockerSecret()
	d := r.TestResourceData()
	d.SetId("sec_1")
	_ = d.Set("endpoint_id", 1)

	if err := r.Delete(d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if mock.FindRequest("DELETE", "/endpoints/1/docker/secrets/sec_1") == nil {
		t.Error("expected DELETE /endpoints/1/docker/secrets/sec_1")
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestDockerSecretCreate_HTTPError verifies that a 5xx response is surfaced.
func TestDockerSecretCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/secrets", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/endpoints/1/docker/secrets/create", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"boom"}`,
	))

	r := resourceDockerSecret()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "x")
	_ = d.Set("data", "y")

	err := r.Create(d, mock.Client())
	if err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}
