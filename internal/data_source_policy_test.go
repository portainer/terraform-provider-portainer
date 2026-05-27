package internal

import (
	"net/http"
	"testing"
)

// TestDataSourcePolicyRead_ByID verifies a direct GET when policy_id is set.
func TestDataSourcePolicyRead_ByID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/policies/42", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":                42,
		"Name":              "Block root",
		"EnvironmentType":   "docker",
		"Type":              "security",
		"EnvironmentGroups": []int{1, 2},
		"Data":              map[string]interface{}{"some": "config"},
		"CreatedAt":         "2024-01-01T00:00:00Z",
		"UpdatedAt":         "2024-01-02T00:00:00Z",
	}))

	ds := dataSourcePortainerPolicy()
	d := ds.TestResourceData()
	_ = d.Set("policy_id", 42)

	if err := ds.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "42" {
		t.Errorf("expected ID %q, got %q", "42", d.Id())
	}
	if got := d.Get("name"); got != "Block root" {
		t.Errorf("name: expected %q, got %v", "Block root", got)
	}
	if got := d.Get("environment_type"); got != "docker" {
		t.Errorf("environment_type: expected %q, got %v", "docker", got)
	}
	if got := d.Get("policy_type"); got != "security" {
		t.Errorf("policy_type: expected %q, got %v", "security", got)
	}
	if got := d.Get("created_at"); got != "2024-01-01T00:00:00Z" {
		t.Errorf("created_at mismatch, got %v", got)
	}
	groups, _ := d.Get("environment_groups").([]interface{})
	if len(groups) != 2 {
		t.Errorf("expected 2 environment_groups, got %d", len(groups))
	}
	if got := d.Get("data"); got == "" {
		t.Errorf("data: expected non-empty JSON string, got empty")
	}
}

// TestDataSourcePolicyRead_ByName verifies the list+filter+detail chain when
// policy is looked up by name.
func TestDataSourcePolicyRead_ByName(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/policies", RespondJSON(http.StatusOK, map[string]interface{}{
		"policies": []map[string]interface{}{
			{"Id": 7, "Name": "Block root"},
			{"Id": 8, "Name": "Allow registry"},
		},
	}))
	mock.On("GET", "/policies/8", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":              8,
		"Name":            "Allow registry",
		"EnvironmentType": "docker",
		"Type":            "registry",
	}))

	ds := dataSourcePortainerPolicy()
	d := ds.TestResourceData()
	_ = d.Set("name", "Allow registry")

	if err := ds.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "8" {
		t.Errorf("expected ID %q, got %q", "8", d.Id())
	}
	if got := d.Get("policy_type"); got != "registry" {
		t.Errorf("policy_type: expected %q, got %v", "registry", got)
	}
}

// TestDataSourcePolicyRead_NameNotFound verifies the error path when the
// name does not match anything in the list.
func TestDataSourcePolicyRead_NameNotFound(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/policies", RespondJSON(http.StatusOK, map[string]interface{}{
		"policies": []map[string]interface{}{
			{"Id": 1, "Name": "Other"},
		},
	}))

	ds := dataSourcePortainerPolicy()
	d := ds.TestResourceData()
	_ = d.Set("name", "ghost")

	if err := ds.Read(d, mock.Client()); err == nil {
		t.Fatal("expected error when policy name not found, got nil")
	}
}

// TestDataSourcePolicyRead_HTTPError verifies a 5xx surfaces.
func TestDataSourcePolicyRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/policies/42", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"boom"}`,
	))

	ds := dataSourcePortainerPolicy()
	d := ds.TestResourceData()
	_ = d.Set("policy_id", 42)

	if err := ds.Read(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}
