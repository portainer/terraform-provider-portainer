package internal

import (
	"net/http"
	"testing"
)

// TestDataSourceWebhookRead_HappyPath verifies that the data source lists
// webhooks and filters by resource_id + endpoint_id.
func TestDataSourceWebhookRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/webhooks", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"Id":         1,
			"EndpointId": 5,
			"ResourceId": "container-abc",
			"Token":      "tok-1",
			"Type":       1,
		},
		{
			"Id":         2,
			"EndpointId": 7,
			"ResourceId": "container-xyz",
			"Token":      "tok-2",
			"Type":       2,
		},
	}))

	ds := dataSourceWebhook()
	d := ds.TestResourceData()
	_ = d.Set("resource_id", "container-xyz")
	_ = d.Set("endpoint_id", 7)

	if err := ds.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "2" {
		t.Errorf("expected ID %q, got %q", "2", d.Id())
	}
	if got := d.Get("token"); got != "tok-2" {
		t.Errorf("token: expected %q, got %v", "tok-2", got)
	}
	if got := d.Get("webhook_type"); got != 2 {
		t.Errorf("webhook_type: expected 2, got %v", got)
	}
}

// TestDataSourceWebhookRead_NotFound verifies the error path.
func TestDataSourceWebhookRead_NotFound(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/webhooks", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "EndpointId": 5, "ResourceId": "container-abc", "Token": "t", "Type": 1},
	}))

	ds := dataSourceWebhook()
	d := ds.TestResourceData()
	_ = d.Set("resource_id", "missing")
	_ = d.Set("endpoint_id", 99)

	if err := ds.Read(d, mock.Client()); err == nil {
		t.Fatal("expected error when webhook not found, got nil")
	}
}

// TestDataSourceWebhookRead_HTTPError verifies HTTP errors propagate.
func TestDataSourceWebhookRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/webhooks", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"boom"}`,
	))

	ds := dataSourceWebhook()
	d := ds.TestResourceData()
	_ = d.Set("resource_id", "container-abc")
	_ = d.Set("endpoint_id", 5)

	if err := ds.Read(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}
