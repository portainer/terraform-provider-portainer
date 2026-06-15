package internal

import (
	"net/http"
	"testing"
)

// TestDataSourceTeamRead_HappyPath verifies that the data source lists teams
// via the SDK (GET /teams), filters by name, and sets the ID.
func TestDataSourceTeamRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/teams", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Name": "developers"},
		{"Id": 2, "Name": "ops"},
	}))

	ds := dataSourceTeam()
	d := ds.TestResourceData()
	_ = d.Set("name", "ops")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "2" {
		t.Errorf("expected ID %q, got %q", "2", d.Id())
	}
	if mock.FindRequest("GET", "/teams") == nil {
		t.Error("expected GET /teams to be sent")
	}
}

// TestDataSourceTeamRead_NotFound verifies that an unmatched name returns an
// error.
func TestDataSourceTeamRead_NotFound(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/teams", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Name": "developers"},
	}))

	ds := dataSourceTeam()
	d := ds.TestResourceData()
	_ = d.Set("name", "ghost")

	if err := rcRead(ds, d, mock.Client()); err == nil {
		t.Fatal("expected error when team not found, got nil")
	}
}

// TestDataSourceTeamRead_HTTPError verifies that an HTTP error is propagated.
func TestDataSourceTeamRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/teams", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"boom"}`,
	))

	ds := dataSourceTeam()
	d := ds.TestResourceData()
	_ = d.Set("name", "ops")

	if err := rcRead(ds, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}
