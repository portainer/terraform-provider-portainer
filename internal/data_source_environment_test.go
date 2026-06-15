package internal

import (
	"net/http"
	"testing"
)

// TestDataSourceEnvironmentRead_HappyPath verifies the data source lists
// environments and selects the one whose Name matches the requested filter.
func TestDataSourceEnvironmentRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Name": "dev", "Type": 1, "URL": "tcp://dev:2375", "GroupId": 1},
		{"Id": 5, "Name": "prod", "Type": 2, "URL": "tcp://prod:2375", "GroupId": 3},
	}))

	ds := dataSourceEnvironment()
	d := ds.TestResourceData()
	_ = d.Set("name", "prod")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "5" {
		t.Errorf("expected ID %q, got %q", "5", d.Id())
	}
	if got := d.Get("type"); got != 2 {
		t.Errorf("type: expected 2, got %v", got)
	}
	if got := d.Get("environment_address"); got != "tcp://prod:2375" {
		t.Errorf("environment_address: expected %q, got %v", "tcp://prod:2375", got)
	}
	if got := d.Get("group_id"); got != 3 {
		t.Errorf("group_id: expected 3, got %v", got)
	}
}

// TestDataSourceEnvironmentRead_NotFound verifies that the data source
// returns an error (and does NOT silently clear ID) when no environment matches.
func TestDataSourceEnvironmentRead_NotFound(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Name": "dev", "Type": 1},
	}))

	ds := dataSourceEnvironment()
	d := ds.TestResourceData()
	_ = d.Set("name", "missing")

	err := rcRead(ds, d, mock.Client())
	if err == nil {
		t.Fatal("expected error for missing environment, got nil")
	}
}

// TestDataSourceEnvironmentRead_HTTPError verifies that an HTTP 5xx surfaces
// as an error from the data source Read.
func TestDataSourceEnvironmentRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"boom"}`,
	))

	ds := dataSourceEnvironment()
	d := ds.TestResourceData()
	_ = d.Set("name", "prod")

	if err := rcRead(ds, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}
