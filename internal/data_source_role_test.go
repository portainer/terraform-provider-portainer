package internal

import (
	"net/http"
	"strconv"
	"testing"
)

// TestDataSourceRoleRead_AllRoles verifies that without a name filter the
// data source returns every role from the list.
func TestDataSourceRoleRead_AllRoles(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/roles", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Name": "endpoint-administrator", "Description": "Full control", "Priority": 1},
		{"Id": 2, "Name": "helpdesk", "Description": "Read-only", "Priority": 2},
		{"Id": 3, "Name": "standard-user", "Description": "Standard", "Priority": 3},
	}))

	ds := dataSourceRole()
	d := ds.TestResourceData()

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	roles, ok := d.Get("roles").([]interface{})
	if !ok {
		t.Fatalf("expected roles list, got %T", d.Get("roles"))
	}
	if len(roles) != 3 {
		t.Errorf("expected 3 roles, got %d", len(roles))
	}
	if d.Id() == "" {
		t.Error("expected ID to be set (synthetic timestamp), got empty")
	}
}

// TestDataSourceRoleRead_FilterByName verifies the name filter narrows the
// result to a single role and uses the role ID as the resource ID.
func TestDataSourceRoleRead_FilterByName(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/roles", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Name": "endpoint-administrator", "Description": "Full", "Priority": 1},
		{"Id": 2, "Name": "helpdesk", "Description": "RO", "Priority": 2},
	}))

	ds := dataSourceRole()
	d := ds.TestResourceData()
	_ = d.Set("name", "helpdesk")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	roles, _ := d.Get("roles").([]interface{})
	if len(roles) != 1 {
		t.Fatalf("expected 1 role after filter, got %d", len(roles))
	}
	entry := roles[0].(map[string]interface{})
	if entry["name"] != "helpdesk" {
		t.Errorf("role name: expected %q, got %v", "helpdesk", entry["name"])
	}
	if d.Id() != strconv.Itoa(2) {
		t.Errorf("expected ID %q, got %q", "2", d.Id())
	}
}

// TestDataSourceRoleRead_FilterNoMatch verifies that a name filter with no
// match returns an error.
func TestDataSourceRoleRead_FilterNoMatch(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/roles", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Name": "endpoint-administrator", "Description": "Full", "Priority": 1},
	}))

	ds := dataSourceRole()
	d := ds.TestResourceData()
	_ = d.Set("name", "ghost")

	if err := rcRead(ds, d, mock.Client()); err == nil {
		t.Fatal("expected error when role name not found, got nil")
	}
}

// TestDataSourceRoleRead_HTTPError verifies the error path.
func TestDataSourceRoleRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/roles", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"boom"}`,
	))

	ds := dataSourceRole()
	d := ds.TestResourceData()

	if err := rcRead(ds, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}
