package internal

import (
	"net/http"
	"testing"
)

// TestDataSourceStackRead_HappyPath verifies the list+filter happy path:
// the data source GETs /stacks, finds the matching name+endpoint_id, and
// populates the computed fields plus d.Id().
func TestDataSourceStackRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/stacks", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Name": "other", "EndpointId": 1, "Type": 1, "SwarmId": ""},
		{"Id": 5, "Name": "mystack", "EndpointId": 2, "Type": 2, "SwarmId": "swarm-abc"},
	}))

	ds := dataSourceStack()
	d := ds.TestResourceData()
	_ = d.Set("name", "mystack")
	_ = d.Set("endpoint_id", 2)

	if err := ds.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "5" {
		t.Errorf("expected ID %q, got %q", "5", d.Id())
	}
	if got := d.Get("type"); got != 2 {
		t.Errorf("type: expected 2, got %v", got)
	}
	if got := d.Get("swarm_id"); got != "swarm-abc" {
		t.Errorf("swarm_id: expected %q, got %v", "swarm-abc", got)
	}
}

// TestDataSourceStackRead_NameInWrongEndpoint verifies that matching name in
// a different endpoint does NOT trigger a match — filter is composite.
func TestDataSourceStackRead_NameInWrongEndpoint(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/stacks", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 5, "Name": "mystack", "EndpointId": 2, "Type": 2, "SwarmId": ""},
	}))

	ds := dataSourceStack()
	d := ds.TestResourceData()
	_ = d.Set("name", "mystack")
	_ = d.Set("endpoint_id", 99)

	err := ds.Read(d, mock.Client())
	if err == nil {
		t.Fatal("expected error when name matches but endpoint differs, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after not-found error, got %q", d.Id())
	}
}

// TestDataSourceStackRead_NotFound verifies that a missing stack returns an
// error (data sources surface not-found rather than clearing state).
func TestDataSourceStackRead_NotFound(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/stacks", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	ds := dataSourceStack()
	d := ds.TestResourceData()
	_ = d.Set("name", "missing")
	_ = d.Set("endpoint_id", 1)

	err := ds.Read(d, mock.Client())
	if err == nil {
		t.Fatal("expected error when stack not found, got nil")
	}
}

// TestDataSourceStackRead_HTTPError verifies that an HTTP 5xx response is
// surfaced as an error.
func TestDataSourceStackRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/stacks", RespondString(http.StatusInternalServerError,
		"application/json", `{"message":"boom"}`))

	ds := dataSourceStack()
	d := ds.TestResourceData()
	_ = d.Set("name", "mystack")
	_ = d.Set("endpoint_id", 1)

	err := ds.Read(d, mock.Client())
	if err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}
