package internal

import (
	"net/http"
	"testing"

	"github.com/hashicorp/go-cty/cty"
)

// resource_alerting_rule talks to /observability/alerting/rules:
//   - Create with `rule_id` adopts an existing (predefined) rule: it GETs the
//     rule, PUTs it back with the configuration laid over it and reads it
//     back, so such a test registers PUT and GET.
//   - Create without `rule_id` POSTs a custom rule, takes the ID from the
//     response (or, failing that, from the list by name) and marks the
//     resource `created`.
//   - Read populates state from a flat AlertingRule object.
//   - Update is identical to the adoption call path.
//   - Delete sends DELETE only for a `created` rule and tolerates 404; an
//     adopted or imported rule is only dropped from state.

// TestAlertingRuleCreate_HappyPath exercises the adoption flow: PUT followed
// by GET. We assert the composite ID, the PUT payload envelope, and that the
// Read response hydrates state.
func TestAlertingRuleCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/observability/alerting/rules/42", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/observability/alerting/rules/42", RespondJSON(http.StatusOK, map[string]interface{}{
		"id":                42,
		"name":              "High CPU",
		"description":       "CPU usage high",
		"enabled":           true,
		"severity":          "critical",
		"metricType":        "percentage",
		"conditionOperator": ">",
		"threshold":         90.0,
		"duration":          120,
	}))

	r := resourceAlertingRule()
	d := r.TestResourceData()
	_ = d.Set("rule_id", 42)
	_ = d.Set("enabled", true)
	_ = d.Set("severity", "critical")
	_ = d.Set("metric_type", "percentage")
	_ = d.Set("condition_operator", ">")
	_ = d.Set("threshold", 90.0)
	_ = d.Set("duration", 120)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "42" {
		t.Errorf("expected ID %q, got %q", "42", d.Id())
	}

	put := mock.FindRequest("PUT", "/observability/alerting/rules/42")
	if put == nil {
		t.Fatal("expected PUT /observability/alerting/rules/42 to be sent")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode PUT body: %v", err)
	}
	rule, ok := payload["AlertingRule"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected PUT body to wrap rule in AlertingRule envelope, got %v", payload)
	}
	if got := rule["enabled"]; got != true {
		t.Errorf("AlertingRule.enabled: expected true, got %v", got)
	}
	if got := rule["severity"]; got != "critical" {
		t.Errorf("AlertingRule.severity: expected %q, got %v", "critical", got)
	}
	// JSON numbers come back as float64 after Unmarshal.
	if got := rule["threshold"]; got != float64(90) {
		t.Errorf("AlertingRule.threshold: expected 90, got %v", got)
	}
	if got := rule["id"]; got != float64(42) {
		t.Errorf("AlertingRule.id: expected 42, got %v", got)
	}

	// Read after Update should populate state from the GET.
	if got := d.Get("name"); got != "High CPU" {
		t.Errorf("name: expected %q, got %v", "High CPU", got)
	}
}

// TestAlertingRuleRead_HappyPath verifies that a successful GET populates
// every relevant field.
func TestAlertingRuleRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/observability/alerting/rules/7", RespondJSON(http.StatusOK, map[string]interface{}{
		"id":                7,
		"name":              "Low Memory",
		"description":       "memory headroom low",
		"summary":           "node memory critical",
		"enabled":           true,
		"severity":          "warning",
		"metricType":        "bytes",
		"conditionOperator": "<",
		"threshold":         1024.0,
		"duration":          60,
		"alertManagerID":    3,
		"isEditable":        true,
		"isInternal":        false,
	}))

	r := resourceAlertingRule()
	d := r.TestResourceData()
	d.SetId("7")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Get("name"); got != "Low Memory" {
		t.Errorf("name: expected %q, got %v", "Low Memory", got)
	}
	if got := d.Get("severity"); got != "warning" {
		t.Errorf("severity: expected %q, got %v", "warning", got)
	}
	if got := d.Get("alert_manager_id"); got != 3 {
		t.Errorf("alert_manager_id: expected 3, got %v", got)
	}
	if got := d.Get("is_editable"); got != true {
		t.Errorf("is_editable: expected true, got %v", got)
	}
}

// TestAlertingRuleRead_404_ClearsID confirms a 404 removes the resource.
func TestAlertingRuleRead_404_ClearsID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/observability/alerting/rules/99", RespondString(
		http.StatusNotFound, "application/json",
		`{"message":"rule not found"}`,
	))

	r := resourceAlertingRule()
	d := r.TestResourceData()
	d.SetId("99")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read should swallow 404, got error: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared after 404, got %q", d.Id())
	}
}

// TestAlertingRuleDelete_HappyPath verifies a DELETE is sent for a rule
// Terraform created.
func TestAlertingRuleDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/observability/alerting/rules/5", RespondString(http.StatusNoContent, "", ""))

	r := resourceAlertingRule()
	d := r.TestResourceData()
	d.SetId("5")
	_ = d.Set("created", true)

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/observability/alerting/rules/5") == nil {
		t.Error("expected DELETE /observability/alerting/rules/5 to be sent")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after delete, got %q", d.Id())
	}
}

// TestAlertingRuleCreate_HTTPError verifies that the PUT-error path surfaces
// as a Create error.
func TestAlertingRuleCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/observability/alerting/rules/1", RespondString(
		http.StatusBadRequest, "application/json",
		`{"message":"invalid rule payload"}`,
	))

	r := resourceAlertingRule()
	d := r.TestResourceData()
	_ = d.Set("rule_id", 1)
	_ = d.Set("enabled", true)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
}

// TestAlertingRuleCreate_CustomRule exercises the POST path: the ID comes
// from the response, the body carries the configured fields in the
// AlertingRule envelope, and the resource is marked as created.
func TestAlertingRuleCreate_CustomRule(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/observability/alerting/rules", RespondJSON(http.StatusOK, map[string]interface{}{"id": 31, "name": "Disk almost full"}))
	mock.On("GET", "/observability/alerting/rules/31", RespondJSON(http.StatusOK, map[string]interface{}{
		"id":                        31,
		"name":                      "Disk almost full",
		"enabled":                   true,
		"category":                  "environment",
		"metricType":                "percentage",
		"promqlExpr":                "disk_used_percent",
		"forDurationMinutes":        10,
		"endpointGroupIds":          []int{2},
		"supportedEnvironmentTypes": "kubernetes",
		"tiers": []map[string]interface{}{
			{"severity": "critical", "threshold": 95, "enabled": true, "selector": `mountpoint="/"`},
		},
	}))

	r := resourceAlertingRule()
	d := r.TestResourceData()
	_ = d.Set("name", "Disk almost full")
	_ = d.Set("enabled", true)
	_ = d.Set("category", "environment")
	_ = d.Set("promql_expr", "disk_used_percent")
	_ = d.Set("for_duration_minutes", 10)
	_ = d.Set("endpoint_group_ids", []interface{}{2})
	_ = d.Set("supported_environment_types", "kubernetes")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "31" || d.Get("rule_id") != 31 {
		t.Errorf("expected ID 31, got %q / %v", d.Id(), d.Get("rule_id"))
	}
	if d.Get("created") != true {
		t.Error("a rule created by POST must be marked as created")
	}

	rule, ok := decodeBody(t, mock, "POST", "/observability/alerting/rules")["AlertingRule"].(map[string]interface{})
	if !ok {
		t.Fatal("expected the POST body to wrap the rule in an AlertingRule envelope")
	}
	for key, want := range map[string]interface{}{
		"name": "Disk almost full", "category": "environment", "promqlExpr": "disk_used_percent",
		"forDurationMinutes": float64(10), "supportedEnvironmentTypes": "kubernetes", "enabled": true,
	} {
		if rule[key] != want {
			t.Errorf("AlertingRule.%s: expected %v, got %v", key, want, rule[key])
		}
	}
	if groups, _ := rule["endpointGroupIds"].([]interface{}); len(groups) != 1 || groups[0] != float64(2) {
		t.Errorf("AlertingRule.endpointGroupIds: expected [2], got %v", rule["endpointGroupIds"])
	}
	if _, sent := rule["id"]; sent {
		t.Error("a create must not send an id")
	}

	if got := d.Get("tiers.0.selector"); got != `mountpoint="/"` {
		t.Errorf("tiers.0.selector: expected the selector, got %v", got)
	}
	if got := d.Get("endpoint_group_ids.0"); got != 2 {
		t.Errorf("endpoint_group_ids.0: expected 2, got %v", got)
	}
}

// TestAlertingRuleCreate_CustomRuleIDFromList covers a create response that
// carries no ID: the rule is found in the list by name, the newest custom
// match winning over an older namesake and a built-in rule.
func TestAlertingRuleCreate_CustomRuleIDFromList(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/observability/alerting/rules", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/observability/alerting/rules", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"id": 3, "name": "Queue backlog", "isInternal": true},
		{"id": 12, "name": "Queue backlog"},
		{"id": 40, "name": "Queue backlog"},
		{"id": 41, "name": "Something else"},
	}))
	mock.On("GET", "/observability/alerting/rules/40", RespondJSON(http.StatusOK, map[string]interface{}{"id": 40, "name": "Queue backlog", "enabled": true}))

	r := resourceAlertingRule()
	d := r.TestResourceData()
	_ = d.Set("name", "Queue backlog")
	_ = d.Set("enabled", true)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "40" {
		t.Errorf("expected the newest custom namesake (40), got %q", d.Id())
	}
}

// TestAlertingRuleCreate_CustomRuleNeedsName rejects a custom rule without a
// name before anything is sent.
func TestAlertingRuleCreate_CustomRuleNeedsName(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceAlertingRule()
	d := r.TestResourceData()
	_ = d.Set("enabled", true)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected an error for a custom rule without a name")
	}
	if len(mock.Requests()) != 0 {
		t.Errorf("expected no request, got %d", len(mock.Requests()))
	}
}

// TestAlertingRuleUpdate_OverlaysConfigOnCurrentRule pins the
// read-modify-write update: the PUT carries the rule as Portainer returned it
// (PromQL, templates, tiers - safe whether PUT merges or replaces), overlaid
// with only what the configuration sets. Values the state holds but the
// configuration does not set (endpoint_group_ids here) are not sent, and a
// tier's selector, which Portainer never accepts from a client, is dropped.
func TestAlertingRuleUpdate_OverlaysConfigOnCurrentRule(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/observability/alerting/rules/8", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/observability/alerting/rules/8", RespondJSON(http.StatusOK, map[string]interface{}{
		"id": 8, "name": "Node NotReady", "enabled": true, "threshold": 5,
		"promqlExpr": "up == 0", "promqlTemplate": "kube_node_status{{.X}}",
		"endpointGroupIds": []int{7},
		"tiers": []map[string]interface{}{
			{"severity": "critical", "threshold": 95, "enabled": true, "selector": `mountpoint="/"`},
		},
	}))

	r := resourceAlertingRule()
	d := dataWithRawConfig(r, map[string]string{
		"rule_id": "8", "enabled": "false", "name": "Node NotReady", "threshold": "0",
		"promql_expr": "up == 0", "endpoint_group_ids.#": "1", "endpoint_group_ids.0": "4",
	}, map[string]cty.Value{
		"rule_id":   cty.NumberIntVal(8),
		"enabled":   cty.False,
		"threshold": cty.NumberIntVal(0),
	})
	d.SetId("8")

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	rule, _ := decodeBody(t, mock, "PUT", "/observability/alerting/rules/8")["AlertingRule"].(map[string]interface{})
	for key, want := range map[string]interface{}{
		"id": float64(8), "enabled": false, "threshold": float64(0), "name": "Node NotReady",
		"promqlExpr": "up == 0", "promqlTemplate": "kube_node_status{{.X}}",
	} {
		if rule[key] != want {
			t.Errorf("AlertingRule.%s: expected %v, got %v", key, want, rule[key])
		}
	}
	if groups, _ := rule["endpointGroupIds"].([]interface{}); len(groups) != 1 || groups[0] != float64(7) {
		t.Errorf("endpointGroupIds: expected Portainer's [7], not the unconfigured state value, got %v", rule["endpointGroupIds"])
	}
	tiers, _ := rule["tiers"].([]interface{})
	if len(tiers) != 1 {
		t.Fatalf("expected the tier to be sent back, got %v", rule["tiers"])
	}
	tier := tiers[0].(map[string]interface{})
	if tier["threshold"] != float64(95) {
		t.Errorf("tier threshold: expected 95, got %v", tier["threshold"])
	}
	if _, sent := tier["selector"]; sent {
		t.Error("a tier's selector must not be sent")
	}
}

// TestAlertingRuleDelete_AdoptedRuleStays verifies that destroying an
// adopted or imported rule sends nothing: Portainer keeps the rule.
func TestAlertingRuleDelete_AdoptedRuleStays(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceAlertingRule()
	d := r.TestResourceData()
	d.SetId("5")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if len(mock.Requests()) != 0 {
		t.Errorf("expected no request for an adopted rule, got %d", len(mock.Requests()))
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after delete, got %q", d.Id())
	}
}
