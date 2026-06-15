package internal

import (
	"net/http"
	"testing"
)

// =========================================================================
// Additional coverage for resource_alerting_settings.go: Read 404 -> clear ID,
// Read non-OK error, Read empty-list -> clear ID, the Update path (reuses
// Create), Delete error propagation, and buildAlertingSettingsPayload with a
// notification channel (incl. config map).
// =========================================================================

// TestAlertingSettingsRead_404ClearsID verifies a 404 clears the resource ID.
func TestAlertingSettingsRead_404ClearsID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/observability/alerting/settings", RespondString(
		http.StatusNotFound, "application/json", `{"message":"not found"}`,
	))

	r := resourceAlertingSettings()
	d := r.TestResourceData()
	d.SetId("5")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read should swallow 404, got: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared after 404, got %q", d.Id())
	}
}

// TestAlertingSettingsRead_HTTPError verifies a non-OK, non-404 status errors.
func TestAlertingSettingsRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/observability/alerting/settings", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourceAlertingSettings()
	d := r.TestResourceData()
	d.SetId("5")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500 read, got nil")
	}
}

// TestAlertingSettingsRead_EmptyListClearsID verifies that an empty list
// response clears the ID (no matching settings entry).
func TestAlertingSettingsRead_EmptyListClearsID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/observability/alerting/settings", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	r := resourceAlertingSettings()
	d := r.TestResourceData()
	d.SetId("5")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared for empty list, got %q", d.Id())
	}
}

// TestAlertingSettingsUpdate_DelegatesToCreate verifies Update reuses the
// Create code path (PUT + chained Read).
func TestAlertingSettingsUpdate_DelegatesToCreate(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/observability/alerting/settings", RespondJSON(http.StatusOK, map[string]interface{}{
		"id": 7, "enabled": true,
	}))
	mock.On("GET", "/observability/alerting/settings", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"id": 7, "enabled": true, "name": "upd"},
	}))

	r := resourceAlertingSettings()
	d := r.TestResourceData()
	d.SetId("7")
	_ = d.Set("enabled", true)
	_ = d.Set("name", "upd")

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if mock.FindRequest("PUT", "/observability/alerting/settings") == nil {
		t.Error("expected PUT /observability/alerting/settings on Update")
	}
	if got := d.Get("name"); got != "upd" {
		t.Errorf("name: expected upd, got %v", got)
	}
}

// TestAlertingSettingsDelete_HTTPError verifies an error on the disabling PUT
// propagates.
func TestAlertingSettingsDelete_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/observability/alerting/settings", RespondString(
		http.StatusBadRequest, "application/json", `{"message":"cannot disable"}`,
	))

	r := resourceAlertingSettings()
	d := r.TestResourceData()
	d.SetId("9")
	_ = d.Set("enabled", true)

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 400 delete, got nil")
	}
}

// TestAlertingSettingsCreate_WithNotificationChannel covers
// buildAlertingSettingsPayload's notification_channels branch, including the
// config map, and verifies the channel is carried in the PUT envelope.
func TestAlertingSettingsCreate_WithNotificationChannel(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/observability/alerting/settings", RespondJSON(http.StatusOK, map[string]interface{}{
		"id": 12, "enabled": true,
	}))
	mock.On("GET", "/observability/alerting/settings", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"id": 12, "enabled": true},
	}))

	r := resourceAlertingSettings()
	d := r.TestResourceData()
	_ = d.Set("enabled", true)
	_ = d.Set("notification_channels", []interface{}{
		map[string]interface{}{
			"name":    "ops-slack",
			"type":    "slack",
			"enabled": true,
			"config": map[string]interface{}{
				"webhook_url": "https://hooks.slack.com/x",
			},
		},
	})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/observability/alerting/settings")
	if put == nil {
		t.Fatal("expected PUT /observability/alerting/settings")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode PUT body: %v", err)
	}
	settings, ok := payload["AlertingSettings"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected AlertingSettings envelope, got %v", payload)
	}
	channels, ok := settings["notificationChannels"].([]interface{})
	if !ok || len(channels) != 1 {
		t.Fatalf("expected 1 notificationChannel in payload, got %v", settings["notificationChannels"])
	}
	ch := channels[0].(map[string]interface{})
	if got := ch["name"]; got != "ops-slack" {
		t.Errorf("channel name: expected ops-slack, got %v", got)
	}
	if got := ch["type"]; got != "slack" {
		t.Errorf("channel type: expected slack, got %v", got)
	}
}
