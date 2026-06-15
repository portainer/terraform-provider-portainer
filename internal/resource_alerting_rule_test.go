package internal

import (
	"net/http"
	"testing"
)

// resource_alerting_rule uses direct http.NewRequest against
// /observability/alerting/rules/{id}:
//   - Create has no POST endpoint: rules are predefined; Create adopts a rule
//     by `rule_id` and then PUTs the updates. After PUT, the resource reads
//     back the rule. So a Create test must register PUT and the follow-up GET.
//   - Read populates state from a flat AlertingRule object.
//   - Update is identical to the Create call path (no POST).
//   - Delete is DELETE /observability/alerting/rules/{id} and tolerates 404.

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

// TestAlertingRuleDelete_HappyPath verifies a DELETE is sent.
func TestAlertingRuleDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/observability/alerting/rules/5", RespondString(http.StatusNoContent, "", ""))

	r := resourceAlertingRule()
	d := r.TestResourceData()
	d.SetId("5")

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
