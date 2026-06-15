package internal

import (
	"net/http"
	"testing"
)

// TestDataSourceEndpointGroupRead_HappyPath verifies that the data source
// selects the matching endpoint group from the list by name.
func TestDataSourceEndpointGroupRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoint_groups", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Name": "default", "Description": "Unassigned"},
		{"Id": 7, "Name": "infra", "Description": "Infra hosts"},
	}))

	ds := dataSourceEndpointGroup()
	d := ds.TestResourceData()
	_ = d.Set("name", "infra")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "7" {
		t.Errorf("expected ID %q, got %q", "7", d.Id())
	}
	if got := d.Get("description"); got != "Infra hosts" {
		t.Errorf("description: expected %q, got %v", "Infra hosts", got)
	}
}

// TestDataSourceEndpointGroupRead_NotFound returns an error when no group matches.
func TestDataSourceEndpointGroupRead_NotFound(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoint_groups", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Name": "default", "Description": "Unassigned"},
	}))

	ds := dataSourceEndpointGroup()
	d := ds.TestResourceData()
	_ = d.Set("name", "ghost")

	if err := rcRead(ds, d, mock.Client()); err == nil {
		t.Fatal("expected error for missing endpoint group, got nil")
	}
}

// TestDataSourceEndpointGroupRead_HTTPError propagates non-2xx responses.
func TestDataSourceEndpointGroupRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoint_groups", RespondString(
		http.StatusForbidden, "application/json",
		`{"message":"nope"}`,
	))

	ds := dataSourceEndpointGroup()
	d := ds.TestResourceData()
	_ = d.Set("name", "infra")

	if err := rcRead(ds, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 403, got nil")
	}
}
