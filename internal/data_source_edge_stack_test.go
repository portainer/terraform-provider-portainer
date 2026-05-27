package internal

import (
	"net/http"
	"testing"
)

// TestDataSourceEdgeStackRead_HappyPath verifies list+filter on /edge_stacks by Name
// and populates the deployment_type computed field.
func TestDataSourceEdgeStackRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/edge_stacks", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Name": "other", "DeploymentType": 0},
		{"Id": 8, "Name": "my-edge-stack", "DeploymentType": 2},
	}))

	ds := dataSourceEdgeStack()
	d := ds.TestResourceData()
	_ = d.Set("name", "my-edge-stack")

	if err := ds.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "8" {
		t.Errorf("expected ID %q, got %q", "8", d.Id())
	}
	if got := d.Get("deployment_type"); got != 2 {
		t.Errorf("deployment_type: expected 2, got %v", got)
	}
}

// TestDataSourceEdgeStackRead_NotFound verifies that a missing name returns an
// error.
func TestDataSourceEdgeStackRead_NotFound(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/edge_stacks", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	ds := dataSourceEdgeStack()
	d := ds.TestResourceData()
	_ = d.Set("name", "missing")

	if err := ds.Read(d, mock.Client()); err == nil {
		t.Fatal("expected error when edge stack not found, got nil")
	}
}

// TestDataSourceEdgeStackRead_HTTPError verifies non-200 status is surfaced.
func TestDataSourceEdgeStackRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/edge_stacks", RespondString(http.StatusBadGateway,
		"application/json", `{"message":"bad gateway"}`))

	ds := dataSourceEdgeStack()
	d := ds.TestResourceData()
	_ = d.Set("name", "x")

	if err := ds.Read(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 502, got nil")
	}
}
