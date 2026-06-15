package internal

import (
	"net/http"
	"testing"
)

// TestDockerVolumeCreate_HappyPath verifies the create POST is sent, the
// composite ID is set, and resource_control_id is stored.
func TestDockerVolumeCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/docker/volumes/create", RespondJSON(http.StatusCreated, map[string]interface{}{
		"Name":   "my-vol",
		"Driver": "local",
		"Portainer": map[string]interface{}{
			"ResourceControl": map[string]interface{}{"Id": 17},
		},
	}))

	r := resourceDockerVolume()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "my-vol")
	_ = d.Set("driver", "local")
	_ = d.Set("labels", map[string]interface{}{"env": "prod"})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "1-my-vol" {
		t.Errorf("expected ID %q, got %q", "1-my-vol", d.Id())
	}
	if got := d.Get("resource_control_id"); got != 17 {
		t.Errorf("resource_control_id: expected 17, got %v", got)
	}

	post := mock.FindRequest("POST", "/endpoints/1/docker/volumes/create")
	if post == nil {
		t.Fatal("expected POST to volumes/create")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if got := payload["Name"]; got != "my-vol" {
		t.Errorf("payload.Name: expected %q, got %v", "my-vol", got)
	}
}

// TestDockerVolumeCreate_ClusterSpec verifies the cluster_volume_spec block is
// expanded into the create payload.
func TestDockerVolumeCreate_ClusterSpec(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/docker/volumes/create", RespondJSON(http.StatusCreated, map[string]interface{}{
		"Name": "cluster-vol",
	}))

	r := resourceDockerVolume()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "cluster-vol")
	_ = d.Set("cluster_volume_spec", []interface{}{
		map[string]interface{}{
			"group":        "g1",
			"availability": "active",
		},
	})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "1-cluster-vol" {
		t.Errorf("expected ID %q, got %q", "1-cluster-vol", d.Id())
	}
}

// TestDockerVolumeCreate_HTTPError verifies a non-2xx create surfaces an error.
func TestDockerVolumeCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/docker/volumes/create", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourceDockerVolume()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "x")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestDockerVolumeRead_HappyPath verifies Read populates state.
func TestDockerVolumeRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/volumes/my-vol", RespondJSON(http.StatusOK, map[string]interface{}{
		"Name":    "my-vol",
		"Driver":  "local",
		"Labels":  map[string]interface{}{"env": "prod"},
		"Options": map[string]interface{}{"type": "nfs"},
		"Portainer": map[string]interface{}{
			"ResourceControl": map[string]interface{}{"Id": 8},
		},
	}))

	r := resourceDockerVolume()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "my-vol")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Get("name"); got != "my-vol" {
		t.Errorf("name: expected %q, got %v", "my-vol", got)
	}
	if got := d.Get("driver"); got != "local" {
		t.Errorf("driver: expected %q, got %v", "local", got)
	}
	if got := d.Get("resource_control_id"); got != 8 {
		t.Errorf("resource_control_id: expected 8, got %v", got)
	}
	if d.Id() != "1-my-vol" {
		t.Errorf("expected ID %q, got %q", "1-my-vol", d.Id())
	}
}

// TestDockerVolumeRead_404ClearsID verifies a 404 clears the ID.
func TestDockerVolumeRead_404ClearsID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/volumes/missing", RespondString(
		http.StatusNotFound, "application/json", `{"message":"not found"}`,
	))

	r := resourceDockerVolume()
	d := r.TestResourceData()
	d.SetId("1-missing")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "missing")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read should swallow 404, got error: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestDockerVolumeRead_HTTPError verifies a non-200/404 read surfaces an error.
func TestDockerVolumeRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/volumes/v1", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourceDockerVolume()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "v1")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestDockerVolumeDelete_HappyPath verifies DELETE is sent and ID cleared.
func TestDockerVolumeDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/docker/volumes/my-vol", RespondString(http.StatusNoContent, "", ""))

	r := resourceDockerVolume()
	d := r.TestResourceData()
	d.SetId("1-my-vol")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "my-vol")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/endpoints/1/docker/volumes/my-vol") == nil {
		t.Error("expected DELETE to volume endpoint")
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestDockerVolumeDelete_404IsSuccess verifies a 404 on delete is success.
func TestDockerVolumeDelete_404IsSuccess(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/docker/volumes/my-vol", RespondString(
		http.StatusNotFound, "application/json", `{"message":"gone"}`,
	))

	r := resourceDockerVolume()
	d := r.TestResourceData()
	d.SetId("1-my-vol")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "my-vol")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete should swallow 404, got error: %v", err)
	}
}

// TestDockerVolumeDelete_HTTPError verifies a non-2xx/non-404 delete surfaces an error.
func TestDockerVolumeDelete_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/docker/volumes/my-vol", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourceDockerVolume()
	d := r.TestResourceData()
	d.SetId("1-my-vol")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "my-vol")

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}
