package internal

import (
	"net/http"
	"strings"
	"testing"
)

// TestAlertingRulesDataSource_Filters lists the rules, keeps the ones
// matching name and category, and orders them by ID.
func TestAlertingRulesDataSource_Filters(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/observability/alerting/rules", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"id": 9, "name": "Node NotReady", "category": "environment", "isInternal": true, "metricType": "days"},
		{"id": 4, "name": "Node NotReady", "category": "environment", "tagIds": []int{3}},
		{"id": 2, "name": "Node NotReady", "category": "security"},
		{"id": 1, "name": "Backup Failure", "category": "portainer"},
	}))

	r := dataSourceAlertingRules()
	d := r.TestResourceData()
	_ = d.Set("name", "Node NotReady")
	_ = d.Set("category", "environment")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if n := d.Get("rules.#"); n != 2 {
		t.Fatalf("expected 2 rules, got %v", n)
	}
	if d.Get("rules.0.rule_id") != 4 || d.Get("rules.1.rule_id") != 9 {
		t.Errorf("expected rules 4 and 9 in ID order, got %v and %v", d.Get("rules.0.rule_id"), d.Get("rules.1.rule_id"))
	}
	if d.Get("rules.0.tag_ids.0") != 3 || d.Get("rules.1.metric_type") != "days" || d.Get("rules.1.is_internal") != true {
		t.Errorf("rule attributes not flattened: %v", d.Get("rules"))
	}
}

// TestAlertingRulesDataSource_ElemIsReadOnly pins that the list element is
// the resource schema with nothing settable and without `created`.
func TestAlertingRulesDataSource_ElemIsReadOnly(t *testing.T) {
	elem := alertingRuleListElemSchema()
	if _, ok := elem["created"]; ok {
		t.Error("the list element must not carry created")
	}
	for key, s := range elem {
		if !s.Computed || s.Optional || s.Required || s.ForceNew || s.ValidateFunc != nil {
			t.Errorf("%s must be computed only", key)
		}
	}
	// The copies must not leak into the resource schema.
	if !resourceAlertingRule().Schema["rule_id"].Optional {
		t.Error("the resource's rule_id must stay optional")
	}
}

// TestObservabilityEnvironmentDataSource_HappyPath maps the discovery
// response.
func TestObservabilityEnvironmentDataSource_HappyPath(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/observability/environments/3", RespondJSON(http.StatusOK, map[string]interface{}{
		"enabled": true, "provider": "oneuptime", "url": "https://oneuptime.example.com",
		"projectId": "p-1", "clusterId": "c-9", "clusterName": "prod",
	}))

	r := dataSourceObservabilityEnvironment()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 3)

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	for key, want := range map[string]interface{}{
		"enabled": true, "provider_name": "oneuptime", "url": "https://oneuptime.example.com",
		"project_id": "p-1", "cluster_id": "c-9", "cluster_name": "prod",
	} {
		if got := d.Get(key); got != want {
			t.Errorf("%s: expected %v, got %v", key, want, got)
		}
	}
}

// TestObservabilityEnvironmentDataSource_OldServer explains the 404 of a
// server older than 2.45.
func TestObservabilityEnvironmentDataSource_OldServer(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{
		"ServerVersion": "2.39.1", "ServerEdition": "BE",
	}))

	r := dataSourceObservabilityEnvironment()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 3)

	err := rcRead(r, d, mock.Client())
	if err == nil || !strings.Contains(err.Error(), "requires Portainer Business Edition 2.45 or later") {
		t.Fatalf("expected the requirement to be named, got %v", err)
	}
}
