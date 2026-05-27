package internal

import (
	"net/http"
	"testing"
)

// TestDataSourceDockerNodeRead_HappyPath matches a node by Description.Hostname
// and exposes role/status from the Swarm node payload.
func TestDataSourceDockerNodeRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/9/docker/nodes", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"ID":          "node-mgr-1",
			"Description": map[string]interface{}{"Hostname": "mgr-1"},
			"Spec":        map[string]interface{}{"Role": "manager"},
			"Status":      map[string]interface{}{"State": "ready"},
		},
		{
			"ID":          "node-wkr-1",
			"Description": map[string]interface{}{"Hostname": "worker-1"},
			"Spec":        map[string]interface{}{"Role": "worker"},
			"Status":      map[string]interface{}{"State": "ready"},
		},
	}))

	ds := dataSourceDockerNode()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 9)
	_ = d.Set("hostname", "worker-1")

	if err := ds.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "node-wkr-1" {
		t.Errorf("expected ID %q, got %q", "node-wkr-1", d.Id())
	}
	if got := d.Get("role"); got != "worker" {
		t.Errorf("role: expected %q, got %v", "worker", got)
	}
	if got := d.Get("status"); got != "ready" {
		t.Errorf("status: expected %q, got %v", "ready", got)
	}
}

// TestDataSourceDockerNodeRead_NotFound errors out if no node matches.
func TestDataSourceDockerNodeRead_NotFound(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/9/docker/nodes", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"ID":          "node-mgr-1",
			"Description": map[string]interface{}{"Hostname": "mgr-1"},
			"Spec":        map[string]interface{}{"Role": "manager"},
			"Status":      map[string]interface{}{"State": "ready"},
		},
	}))

	ds := dataSourceDockerNode()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 9)
	_ = d.Set("hostname", "ghost")

	if err := ds.Read(d, mock.Client()); err == nil {
		t.Fatal("expected error for missing docker node, got nil")
	}
}

// TestDataSourceDockerNodeRead_HTTPError surfaces non-Swarm/cluster errors.
func TestDataSourceDockerNodeRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/9/docker/nodes", RespondString(
		http.StatusServiceUnavailable, "application/json",
		`{"message":"not a swarm manager"}`,
	))

	ds := dataSourceDockerNode()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 9)
	_ = d.Set("hostname", "mgr-1")

	if err := ds.Read(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 503, got nil")
	}
}
