package internal

import (
	"net/http"
	"testing"
)

// resource_alerting_settings uses direct http.NewRequest:
//   - Create PUTs /observability/alerting/settings with the AlertingSettings
//     wrapped in {"AlertingSettings": {...}}. If the response carries an "id"
//     field the resource sets that as the ID; otherwise it falls back to the
//     literal "portainer-alerting-settings".
//   - Read GETs /observability/alerting/settings and supports both an array
//     and a single-object payload shape, finding the entry by ID or
//     defaulting to the first item.
//   - Update reuses the Create code path.
//   - Delete sends a PUT with enabled=false (it disables instead of removing).

// TestAlertingSettingsCreate_HappyPath verifies the Create envelope, that the
// resource ID is parsed from the response id field, and that the follow-up
// Read populates state.
func TestAlertingSettingsCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	// First call: Create's PUT.
	// Second call: Read's GET.
	mock.On("PUT", "/observability/alerting/settings", RespondJSON(http.StatusOK, map[string]interface{}{
		"id":      4,
		"enabled": true,
	}))
	mock.On("GET", "/observability/alerting/settings", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"id":           4,
			"name":         "primary-am",
			"enabled":      true,
			"url":          "http://am.example:9093",
			"portainerURL": "http://portainer.example",
			"isInternal":   false,
			"status":       "connected",
		},
	}))

	r := resourceAlertingSettings()
	d := r.TestResourceData()
	_ = d.Set("enabled", true)
	_ = d.Set("name", "primary-am")
	_ = d.Set("url", "http://am.example:9093")
	_ = d.Set("portainer_url", "http://portainer.example")

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "4" {
		t.Errorf("expected ID %q, got %q", "4", d.Id())
	}

	put := mock.FindRequest("PUT", "/observability/alerting/settings")
	if put == nil {
		t.Fatal("expected PUT /observability/alerting/settings to be sent")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode PUT body: %v", err)
	}
	settings, ok := payload["AlertingSettings"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected AlertingSettings envelope, got %v", payload)
	}
	if got := settings["enabled"]; got != true {
		t.Errorf("settings.enabled: expected true, got %v", got)
	}
	if got := settings["url"]; got != "http://am.example:9093" {
		t.Errorf("settings.url: expected %q, got %v", "http://am.example:9093", got)
	}

	// Follow-up Read should have hydrated state.
	if got := d.Get("status"); got != "connected" {
		t.Errorf("status: expected %q, got %v", "connected", got)
	}
}

// TestAlertingSettingsCreate_NoIDFallback verifies the fallback to the
// literal "portainer-alerting-settings" ID when the create response does not
// include an "id". The Read then matches by listing and picks the first item.
func TestAlertingSettingsCreate_NoIDFallback(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/observability/alerting/settings", RespondJSON(http.StatusOK, map[string]interface{}{
		"enabled": true,
	}))
	mock.On("GET", "/observability/alerting/settings", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"id": 11, "enabled": true, "url": "http://am.example"},
	}))

	r := resourceAlertingSettings()
	d := r.TestResourceData()
	_ = d.Set("enabled", true)

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Read promotes the first entry's ID when the placeholder ID didn't match.
	if d.Id() != "11" {
		t.Errorf("expected ID promoted to %q by Read, got %q", "11", d.Id())
	}
}

// TestAlertingSettingsRead_SingleObjectResponse covers the alternative
// response shape: a single object instead of an array.
func TestAlertingSettingsRead_SingleObjectResponse(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/observability/alerting/settings", RespondJSON(http.StatusOK, map[string]interface{}{
		"id":      8,
		"enabled": true,
		"name":    "single-shape",
		"url":     "http://am.example",
		"status":  "disabled",
	}))

	r := resourceAlertingSettings()
	d := r.TestResourceData()
	d.SetId("8")

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("name"); got != "single-shape" {
		t.Errorf("name: expected %q, got %v", "single-shape", got)
	}
	if got := d.Get("status"); got != "disabled" {
		t.Errorf("status: expected %q, got %v", "disabled", got)
	}
}

// TestAlertingSettingsDelete_SendsDisablePUT verifies Delete is implemented
// as a PUT with enabled=false (the API has no DELETE for settings).
func TestAlertingSettingsDelete_SendsDisablePUT(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/observability/alerting/settings", RespondJSON(http.StatusOK, map[string]interface{}{
		"id":      9,
		"enabled": false,
	}))

	r := resourceAlertingSettings()
	d := r.TestResourceData()
	d.SetId("9")
	_ = d.Set("enabled", true)

	if err := r.Delete(d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/observability/alerting/settings")
	if put == nil {
		t.Fatal("expected disabling PUT /observability/alerting/settings")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode PUT body: %v", err)
	}
	settings, ok := payload["AlertingSettings"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected AlertingSettings envelope, got %v", payload)
	}
	// `enabled:false` is encoded as a present-but-false value (JSON tag
	// has no omitempty on Enabled).
	if got, present := settings["enabled"]; !present || got != false {
		t.Errorf("settings.enabled: expected false (present), got %v (present=%v)", got, present)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared after delete, got %q", d.Id())
	}
}

// TestAlertingSettingsCreate_HTTPError verifies error propagation.
func TestAlertingSettingsCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/observability/alerting/settings", RespondString(
		http.StatusBadRequest, "application/json",
		`{"message":"bad payload"}`,
	))

	r := resourceAlertingSettings()
	d := r.TestResourceData()
	_ = d.Set("enabled", true)

	if err := r.Create(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}
