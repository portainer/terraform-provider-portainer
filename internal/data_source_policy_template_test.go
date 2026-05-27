package internal

import (
	"net/http"
	"testing"
)

// TestDataSourcePolicyTemplateRead_ByID verifies the direct GET branch when
// template_id is provided.
func TestDataSourcePolicyTemplateRead_ByID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/policies/templates/no-root-containers", RespondJSON(http.StatusOK, map[string]interface{}{
		"id":          "no-root-containers",
		"name":        "Block root containers",
		"description": "Disallow root user in containers",
		"category":    "security",
		"type":        "security",
		"data":        map[string]interface{}{"rule": "block-root"},
	}))

	ds := dataSourcePortainerPolicyTemplate()
	d := ds.TestResourceData()
	_ = d.Set("template_id", "no-root-containers")

	if err := ds.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "no-root-containers" {
		t.Errorf("expected ID %q, got %q", "no-root-containers", d.Id())
	}
	if got := d.Get("name"); got != "Block root containers" {
		t.Errorf("name: expected %q, got %v", "Block root containers", got)
	}
	if got := d.Get("category"); got != "security" {
		t.Errorf("category: expected %q, got %v", "security", got)
	}
	if got := d.Get("policy_type"); got != "security" {
		t.Errorf("policy_type: expected %q, got %v", "security", got)
	}
	if got := d.Get("data"); got == "" {
		t.Errorf("data: expected non-empty JSON string, got empty")
	}
}

// TestDataSourcePolicyTemplateRead_ByName verifies the list+filter+detail
// chain when name is provided.
func TestDataSourcePolicyTemplateRead_ByName(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/policies/templates", RespondJSON(http.StatusOK, map[string]interface{}{
		"templates": []map[string]interface{}{
			{"id": "rbac-admin", "name": "RBAC admin"},
			{"id": "no-root", "name": "Block root"},
		},
	}))
	mock.On("GET", "/policies/templates/no-root", RespondJSON(http.StatusOK, map[string]interface{}{
		"id":       "no-root",
		"name":     "Block root",
		"category": "security",
		"type":     "security",
	}))

	ds := dataSourcePortainerPolicyTemplate()
	d := ds.TestResourceData()
	_ = d.Set("name", "Block root")

	if err := ds.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "no-root" {
		t.Errorf("expected ID %q, got %q", "no-root", d.Id())
	}
	if got := d.Get("category"); got != "security" {
		t.Errorf("category: expected %q, got %v", "security", got)
	}
}

// TestDataSourcePolicyTemplateRead_NameNotFound verifies the error path.
func TestDataSourcePolicyTemplateRead_NameNotFound(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/policies/templates", RespondJSON(http.StatusOK, map[string]interface{}{
		"templates": []map[string]interface{}{
			{"id": "rbac-admin", "name": "RBAC admin"},
		},
	}))

	ds := dataSourcePortainerPolicyTemplate()
	d := ds.TestResourceData()
	_ = d.Set("name", "ghost")

	if err := ds.Read(d, mock.Client()); err == nil {
		t.Fatal("expected error when template name not found, got nil")
	}
}

// TestDataSourcePolicyTemplateRead_HTTPError verifies HTTP errors propagate.
func TestDataSourcePolicyTemplateRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/policies/templates/missing", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"boom"}`,
	))

	ds := dataSourcePortainerPolicyTemplate()
	d := ds.TestResourceData()
	_ = d.Set("template_id", "missing")

	if err := ds.Read(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}
