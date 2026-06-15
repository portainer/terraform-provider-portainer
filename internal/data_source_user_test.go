package internal

import (
	"net/http"
	"testing"
)

// TestDataSourceUserRead_HappyPath verifies that the data source lists users
// via the SDK (GET /users), filters by username, and populates ID + role.
func TestDataSourceUserRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/users", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Username": "admin", "Role": 1},
		{"Id": 5, "Username": "alice", "Role": 2},
		{"Id": 6, "Username": "bob", "Role": 2},
	}))

	ds := dataSourceUser()
	d := ds.TestResourceData()
	_ = d.Set("username", "alice")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "5" {
		t.Errorf("expected ID %q, got %q", "5", d.Id())
	}
	if got := d.Get("role"); got != 2 {
		t.Errorf("role: expected 2, got %v", got)
	}
	if mock.FindRequest("GET", "/users") == nil {
		t.Error("expected GET /users to be sent")
	}
}

// TestDataSourceUserRead_NotFound verifies that the DS errors out (rather
// than clearing the ID) when no matching user exists.
func TestDataSourceUserRead_NotFound(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/users", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Username": "admin", "Role": 1},
	}))

	ds := dataSourceUser()
	d := ds.TestResourceData()
	_ = d.Set("username", "ghost")

	if err := rcRead(ds, d, mock.Client()); err == nil {
		t.Fatal("expected error when user not found, got nil")
	}
}

// TestDataSourceUserRead_HTTPError verifies that an error response from the
// list endpoint is surfaced.
func TestDataSourceUserRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/users", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"boom"}`,
	))

	ds := dataSourceUser()
	d := ds.TestResourceData()
	_ = d.Set("username", "alice")

	if err := rcRead(ds, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}
