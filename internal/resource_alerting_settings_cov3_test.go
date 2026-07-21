package internal

import (
	"net/http"
	"testing"
)

// TestAlertingSettingsCov3_Read_MatchByIDWithChannels covers the ID-match
// branch of the settings-lookup loop (the resource ID matches a specific entry
// in a multi-entry array) and the notification-channel config mapping branch.
func TestAlertingSettingsCov3_Read_MatchByIDWithChannels(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/observability/alerting/settings", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"id":      1,
			"name":    "first",
			"enabled": false,
		},
		{
			"id":      5,
			"name":    "second",
			"enabled": true,
			"url":     "http://alertmanager:9093",
			"notificationChannels": []map[string]interface{}{
				{
					"id":      11,
					"name":    "ops-slack",
					"type":    "slack",
					"enabled": true,
					"config": map[string]interface{}{
						"webhookURL": "https://hooks.slack.com/x",
					},
				},
			},
		},
	}))

	r := resourceAlertingSettings()
	d := r.TestResourceData()
	d.SetId("5")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "5" {
		t.Errorf("expected matched ID %q, got %q", "5", d.Id())
	}
	if got := d.Get("name"); got != "second" {
		t.Errorf("name: expected matched entry %q, got %v", "second", got)
	}
	if got := d.Get("enabled"); got != true {
		t.Errorf("enabled: expected true, got %v", got)
	}
	channels := d.Get("notification_channels").([]interface{})
	if len(channels) != 1 {
		t.Fatalf("notification_channels: expected 1, got %d", len(channels))
	}
	ch := channels[0].(map[string]interface{})
	if ch["type"] != "slack" {
		t.Errorf("channel.type: got %v", ch["type"])
	}
	cfg := ch["config"].(map[string]interface{})
	if cfg["webhookURL"] != "https://hooks.slack.com/x" {
		t.Errorf("channel.config.webhookURL: got %v", cfg["webhookURL"])
	}
}
