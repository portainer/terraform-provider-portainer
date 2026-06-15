package internal

import (
	"net/http"
	"testing"
)

// TestDataSourceEdgeGroupRead_HappyPath verifies list+filter on /edge_groups
// matches by Name and populates computed fields.
func TestDataSourceEdgeGroupRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/edge_groups", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Name": "other", "Dynamic": false},
		{"Id": 7, "Name": "my-group", "Dynamic": true},
	}))

	ds := dataSourceEdgeGroup()
	d := ds.TestResourceData()
	_ = d.Set("name", "my-group")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "7" {
		t.Errorf("expected ID %q, got %q", "7", d.Id())
	}
	if got := d.Get("dynamic"); got != true {
		t.Errorf("dynamic: expected true, got %v", got)
	}
}

// TestDataSourceEdgeGroupRead_NotFound verifies a missing name returns an error.
func TestDataSourceEdgeGroupRead_NotFound(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/edge_groups", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Name": "other"},
	}))

	ds := dataSourceEdgeGroup()
	d := ds.TestResourceData()
	_ = d.Set("name", "missing")

	if err := rcRead(ds, d, mock.Client()); err == nil {
		t.Fatal("expected error when edge group not found, got nil")
	}
}

// TestDataSourceEdgeGroupRead_HTTPError verifies non-200 status is surfaced.
func TestDataSourceEdgeGroupRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/edge_groups", RespondString(http.StatusForbidden,
		"application/json", `{"message":"forbidden"}`))

	ds := dataSourceEdgeGroup()
	d := ds.TestResourceData()
	_ = d.Set("name", "x")

	if err := rcRead(ds, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 403, got nil")
	}
}
