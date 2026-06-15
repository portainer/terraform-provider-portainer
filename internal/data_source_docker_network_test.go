package internal

import (
	"net/http"
	"testing"
)

// TestDataSourceDockerNetworkRead_HappyPath matches a network by Name and
// exposes the driver and scope as computed fields.
func TestDataSourceDockerNetworkRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/networks", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": "net-1", "Name": "bridge", "Driver": "bridge", "Scope": "local"},
		{"Id": "net-overlay-xyz", "Name": "ingress", "Driver": "overlay", "Scope": "swarm"},
	}))

	ds := dataSourceDockerNetwork()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "ingress")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "net-overlay-xyz" {
		t.Errorf("expected ID %q, got %q", "net-overlay-xyz", d.Id())
	}
	if got := d.Get("driver"); got != "overlay" {
		t.Errorf("driver: expected %q, got %v", "overlay", got)
	}
	if got := d.Get("scope"); got != "swarm" {
		t.Errorf("scope: expected %q, got %v", "swarm", got)
	}
}

// TestDataSourceDockerNetworkRead_NotFound errors out if no network matches.
func TestDataSourceDockerNetworkRead_NotFound(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/networks", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": "net-1", "Name": "bridge", "Driver": "bridge", "Scope": "local"},
	}))

	ds := dataSourceDockerNetwork()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "missing")

	if err := rcRead(ds, d, mock.Client()); err == nil {
		t.Fatal("expected error for missing docker network, got nil")
	}
}

// TestDataSourceDockerNetworkRead_HTTPError propagates HTTP errors.
func TestDataSourceDockerNetworkRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/networks", RespondString(
		http.StatusServiceUnavailable, "application/json",
		`{"message":"down"}`,
	))

	ds := dataSourceDockerNetwork()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "ingress")

	if err := rcRead(ds, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 503, got nil")
	}
}
