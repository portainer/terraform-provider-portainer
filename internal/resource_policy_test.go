package internal

import (
	"net/http"
	"testing"
)

// TestPolicyCreate_HappyPath verifies POST /policies returns an Id and
// triggers a follow-up Read that populates state.
func TestPolicyCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/policies", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 42,
	}))
	mock.On("GET", "/policies/42", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":                42,
		"Name":              "test-policy",
		"EnvironmentType":   "kubernetes",
		"Type":              "rbac-k8s",
		"EnvironmentGroups": []int{1, 2},
		"CreatedAt":         "2026-01-01T00:00:00Z",
		"UpdatedAt":         "2026-01-01T00:00:00Z",
		"Data":              map[string]interface{}{"foo": "bar"},
	}))

	r := resourcePortainerPolicy()
	d := r.TestResourceData()
	_ = d.Set("name", "test-policy")
	_ = d.Set("environment_type", "kubernetes")
	_ = d.Set("policy_type", "rbac-k8s")
	_ = d.Set("environment_groups", []interface{}{1, 2})
	_ = d.Set("data", `{"foo":"bar"}`)
	_ = d.Set("allow_override", true)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "42" {
		t.Errorf("ID: got %q", d.Id())
	}
	if got := d.Get("name"); got != "test-policy" {
		t.Errorf("name: got %v", got)
	}
	if got := d.Get("created_at"); got != "2026-01-01T00:00:00Z" {
		t.Errorf("created_at: got %v", got)
	}

	// Verify POST payload uses the PascalCase keys the API expects.
	post := mock.FindRequest("POST", "/policies")
	if post == nil {
		t.Fatal("expected POST /policies")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if got := payload["Name"]; got != "test-policy" {
		t.Errorf("payload.Name: got %v", got)
	}
	if got := payload["EnvironmentType"]; got != "kubernetes" {
		t.Errorf("payload.EnvironmentType: got %v", got)
	}
	if got := payload["Type"]; got != "rbac-k8s" {
		t.Errorf("payload.Type: got %v", got)
	}
	if got := payload["AllowOverride"]; got != true {
		t.Errorf("payload.AllowOverride: got %v", got)
	}
}

// TestPolicyRead_HappyPath verifies Read populates state from the policy
// response.
func TestPolicyRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/policies/7", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":              7,
		"Name":            "p1",
		"EnvironmentType": "docker",
		"Type":            "security-docker",
	}))

	r := resourcePortainerPolicy()
	d := r.TestResourceData()
	d.SetId("7")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("name"); got != "p1" {
		t.Errorf("name: got %v", got)
	}
	if got := d.Get("environment_type"); got != "docker" {
		t.Errorf("environment_type: got %v", got)
	}
}

// TestPolicyRead_404_ClearsID verifies drift handling on 404.
func TestPolicyRead_404_ClearsID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/policies/99", RespondString(http.StatusNotFound, "application/json", `{"message":"gone"}`))

	r := resourcePortainerPolicy()
	d := r.TestResourceData()
	d.SetId("99")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read should swallow 404, got: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestPolicyUpdate_HappyPath verifies PUT /policies/{id} is sent.
func TestPolicyUpdate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/policies/11", RespondString(http.StatusOK, "application/json", `{}`))
	mock.On("GET", "/policies/11", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":              11,
		"Name":            "renamed",
		"EnvironmentType": "kubernetes",
	}))

	r := resourcePortainerPolicy()
	d := r.TestResourceData()
	d.SetId("11")
	_ = d.Set("name", "renamed")
	_ = d.Set("environment_type", "kubernetes")
	_ = d.Set("policy_type", "rbac-k8s")

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if mock.FindRequest("PUT", "/policies/11") == nil {
		t.Error("expected PUT /policies/11")
	}
}

// TestPolicyDelete_HappyPath verifies DELETE returns 204 successfully.
func TestPolicyDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/policies/5", RespondString(http.StatusNoContent, "", ""))

	r := resourcePortainerPolicy()
	d := r.TestResourceData()
	d.SetId("5")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/policies/5") == nil {
		t.Error("expected DELETE /policies/5")
	}
}

// TestPolicyCreate_HTTPError verifies error propagation.
func TestPolicyCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/policies", RespondString(
		http.StatusBadRequest, "application/json",
		`{"message":"bad"}`,
	))

	r := resourcePortainerPolicy()
	d := r.TestResourceData()
	_ = d.Set("name", "bad")
	_ = d.Set("environment_type", "kubernetes")
	_ = d.Set("policy_type", "rbac-k8s")

	err := rcCreate(r, d, mock.Client())
	if err == nil {
		t.Fatal("expected error on 400, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID, got %q", d.Id())
	}
}
