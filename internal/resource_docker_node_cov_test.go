package internal

import (
	"net/http"
	"testing"
)

// TestDockerNodeUpdate_HappyPath verifies the update POST is sent to the
// version-scoped URL and the composite ID is set.
func TestDockerNodeUpdate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/docker/nodes/node-abc/update", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceDockerNode()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("node_id", "node-abc")
	_ = d.Set("version", 12)
	_ = d.Set("name", "worker-1")
	_ = d.Set("availability", "active")
	_ = d.Set("role", "worker")
	_ = d.Set("labels", map[string]interface{}{"rack": "1"})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update/Create failed: %v", err)
	}

	if d.Id() != "1-node-abc" {
		t.Errorf("expected ID %q, got %q", "1-node-abc", d.Id())
	}

	post := mock.FindRequest("POST", "/endpoints/1/docker/nodes/node-abc/update")
	if post == nil {
		t.Fatal("expected POST to node update endpoint")
	}
	if post.Query != "version=12" {
		t.Errorf("expected query version=12, got %q", post.Query)
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if got := payload["Name"]; got != "worker-1" {
		t.Errorf("payload.Name: expected %q, got %v", "worker-1", got)
	}
	if got := payload["Availability"]; got != "active" {
		t.Errorf("payload.Availability: expected %q, got %v", "active", got)
	}
}

// TestDockerNodeUpdate_HTTPError verifies a non-2xx response surfaces an error.
func TestDockerNodeUpdate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/docker/nodes/n1/update", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourceDockerNode()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("node_id", "n1")
	_ = d.Set("version", 1)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestDockerNodeRead_HappyPath verifies Read populates state from the inspect
// endpoint payload.
func TestDockerNodeRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/nodes/node-abc", RespondJSON(http.StatusOK, map[string]interface{}{
		"ID": "node-abc",
		"Version": map[string]interface{}{
			"Index": 25,
		},
		"Spec": map[string]interface{}{
			"Availability": "drain",
			"Name":         "manager-1",
			"Role":         "manager",
			"Labels":       map[string]interface{}{"zone": "a"},
		},
	}))

	r := resourceDockerNode()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("node_id", "node-abc")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Get("version"); got != 25 {
		t.Errorf("version: expected 25, got %v", got)
	}
	if got := d.Get("name"); got != "manager-1" {
		t.Errorf("name: expected %q, got %v", "manager-1", got)
	}
	if got := d.Get("availability"); got != "drain" {
		t.Errorf("availability: expected %q, got %v", "drain", got)
	}
	if got := d.Get("role"); got != "manager" {
		t.Errorf("role: expected %q, got %v", "manager", got)
	}
	if d.Id() != "1-node-abc" {
		t.Errorf("expected ID %q, got %q", "1-node-abc", d.Id())
	}
}

// TestDockerNodeRead_404ClearsID verifies a 404 on Read clears the ID.
func TestDockerNodeRead_404ClearsID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/nodes/missing", RespondString(
		http.StatusNotFound, "application/json", `{"message":"not found"}`,
	))

	r := resourceDockerNode()
	d := r.TestResourceData()
	d.SetId("1-missing")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("node_id", "missing")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read should swallow 404, got error: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestDockerNodeRead_HTTPError verifies a non-200/404 response surfaces an error.
func TestDockerNodeRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/nodes/n1", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourceDockerNode()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("node_id", "n1")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestDockerNodeDelete_HappyPath verifies DELETE is sent and the ID is cleared.
func TestDockerNodeDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/docker/nodes/node-abc", RespondString(http.StatusOK, "", ""))

	r := resourceDockerNode()
	d := r.TestResourceData()
	d.SetId("1-node-abc")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("node_id", "node-abc")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if mock.FindRequest("DELETE", "/endpoints/1/docker/nodes/node-abc") == nil {
		t.Error("expected DELETE to node endpoint")
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestDockerNodeDelete_HTTPError verifies a non-2xx delete surfaces an error.
func TestDockerNodeDelete_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/docker/nodes/n1", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourceDockerNode()
	d := r.TestResourceData()
	d.SetId("1-n1")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("node_id", "n1")

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}
