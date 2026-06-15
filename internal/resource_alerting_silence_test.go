package internal

import (
	"net/http"
	"strings"
	"testing"
)

// resource_alerting_silence uses direct http.NewRequest:
//   - Create POSTs JSON to /observability/alerting/silence and expects either
//     {"silenceID":"..."} or {"id":"..."} in the response. After Create, the
//     Read step is invoked to verify the silence exists.
//   - Read GETs /observability/alerting/alerts?status=silenced — the resource
//     does NOT parse the response, only that the call succeeds. So a stub
//     200 returning an empty list is enough.
//   - There is no Update; every schema field is ForceNew.
//   - Delete DELETEs /observability/alerting/silence/{id}?alertManagerURL=...
//     and tolerates 404.

// TestAlertingSilenceCreate_HappyPath_SilenceIDKey covers the path where
// Portainer returns {"silenceID":"<uuid>"} — this is the documented response
// shape. The follow-up Read just needs the alerts endpoint to return 200.
func TestAlertingSilenceCreate_HappyPath_SilenceIDKey(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/observability/alerting/silence", RespondJSON(http.StatusOK, map[string]interface{}{
		"silenceID": "abc-123",
	}))
	mock.On("GET", "/observability/alerting/alerts", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	r := resourceAlertingSilence()
	d := r.TestResourceData()
	_ = d.Set("alert_manager_url", "http://am.example:9093")
	_ = d.Set("comment", "maintenance window")
	_ = d.Set("created_by", "ci")
	_ = d.Set("starts_at", "2026-01-01T00:00:00Z")
	_ = d.Set("ends_at", "2026-01-02T00:00:00Z")
	_ = d.Set("matchers", []interface{}{
		map[string]interface{}{
			"name":     "alertname",
			"value":    "HighCPU",
			"is_regex": false,
			"is_equal": true,
		},
	})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "abc-123" {
		t.Errorf("expected ID %q, got %q", "abc-123", d.Id())
	}

	post := mock.FindRequest("POST", "/observability/alerting/silence")
	if post == nil {
		t.Fatal("expected POST /observability/alerting/silence to be sent")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode POST body: %v", err)
	}
	if got := payload["alertManagerURL"]; got != "http://am.example:9093" {
		t.Errorf("alertManagerURL: expected %q, got %v", "http://am.example:9093", got)
	}
	silence, ok := payload["silence"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected silence envelope, got %v", payload["silence"])
	}
	if got := silence["comment"]; got != "maintenance window" {
		t.Errorf("silence.comment: expected %q, got %v", "maintenance window", got)
	}
	if got := silence["createdBy"]; got != "ci" {
		t.Errorf("silence.createdBy: expected %q, got %v", "ci", got)
	}
	matchers, ok := silence["matchers"].([]interface{})
	if !ok || len(matchers) != 1 {
		t.Fatalf("silence.matchers: expected one matcher, got %v", silence["matchers"])
	}
	m := matchers[0].(map[string]interface{})
	if got := m["name"]; got != "alertname" {
		t.Errorf("matcher.name: expected %q, got %v", "alertname", got)
	}
	if got := m["value"]; got != "HighCPU" {
		t.Errorf("matcher.value: expected %q, got %v", "HighCPU", got)
	}
	if got := m["isRegex"]; got != false {
		t.Errorf("matcher.isRegex: expected false, got %v", got)
	}
}

// TestAlertingSilenceCreate_HappyPath_IDKey covers the fallback parser branch
// where the response uses {"id":"<uuid>"} instead of {"silenceID":"..."}.
func TestAlertingSilenceCreate_HappyPath_IDKey(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/observability/alerting/silence", RespondJSON(http.StatusOK, map[string]interface{}{
		"id": "xyz-9",
	}))
	mock.On("GET", "/observability/alerting/alerts", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	r := resourceAlertingSilence()
	d := r.TestResourceData()
	_ = d.Set("alert_manager_url", "http://am.example")
	_ = d.Set("comment", "x")
	_ = d.Set("created_by", "x")
	_ = d.Set("starts_at", "2026-01-01T00:00:00Z")
	_ = d.Set("ends_at", "2026-01-02T00:00:00Z")
	_ = d.Set("matchers", []interface{}{
		map[string]interface{}{
			"name":     "k",
			"value":    "v",
			"is_regex": false,
			"is_equal": true,
		},
	})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "xyz-9" {
		t.Errorf("expected ID %q, got %q", "xyz-9", d.Id())
	}
}

// TestAlertingSilenceDelete_HappyPath verifies the DELETE call carries the
// silence ID in the path and the alertManagerURL in the query string.
func TestAlertingSilenceDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/observability/alerting/silence/abc-123",
		RespondString(http.StatusNoContent, "", ""))

	r := resourceAlertingSilence()
	d := r.TestResourceData()
	d.SetId("abc-123")
	_ = d.Set("alert_manager_url", "http://am.example:9093")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	del := mock.FindRequest("DELETE", "/observability/alerting/silence/abc-123")
	if del == nil {
		t.Fatal("expected DELETE /observability/alerting/silence/abc-123 to be sent")
	}
	if !strings.Contains(del.Query, "alertManagerURL=") {
		t.Errorf("expected query to contain alertManagerURL=, got %q", del.Query)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared after delete, got %q", d.Id())
	}
}

// TestAlertingSilenceDelete_404_NoError confirms the 404 swallow behavior.
func TestAlertingSilenceDelete_404_NoError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/observability/alerting/silence/gone", RespondString(
		http.StatusNotFound, "application/json",
		`{"message":"silence not found"}`,
	))

	r := resourceAlertingSilence()
	d := r.TestResourceData()
	d.SetId("gone")
	_ = d.Set("alert_manager_url", "http://am.example")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete should swallow 404, got error: %v", err)
	}
}

// TestAlertingSilenceCreate_HTTPError verifies that a 4xx response from
// /observability/alerting/silence propagates as an error.
func TestAlertingSilenceCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/observability/alerting/silence", RespondString(
		http.StatusBadRequest, "application/json",
		`{"message":"invalid silence"}`,
	))

	r := resourceAlertingSilence()
	d := r.TestResourceData()
	_ = d.Set("alert_manager_url", "http://am.example")
	_ = d.Set("comment", "x")
	_ = d.Set("created_by", "x")
	_ = d.Set("starts_at", "2026-01-01T00:00:00Z")
	_ = d.Set("ends_at", "2026-01-02T00:00:00Z")
	_ = d.Set("matchers", []interface{}{
		map[string]interface{}{
			"name":     "k",
			"value":    "v",
			"is_regex": false,
			"is_equal": true,
		},
	})

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}
