package internal

import (
	"net/http"
	"testing"
)

// TestWebhookCreate_HappyPath verifies POST /webhooks sends the expected
// payload and that the returned ID + token populate state.
func TestWebhookCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/webhooks", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":         77,
		"EndpointId": 1,
		"ResourceId": "abc123",
		"Type":       1,
		"Token":      "deadbeef-token",
	}))

	r := resourceWebhook()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("resource_id", "abc123")
	_ = d.Set("webhook_type", 1)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "77" {
		t.Errorf("expected ID %q, got %q", "77", d.Id())
	}
	if got := d.Get("token"); got != "deadbeef-token" {
		t.Errorf("token: expected %q, got %v", "deadbeef-token", got)
	}

	post := mock.FindRequest("POST", "/webhooks")
	if post == nil {
		t.Fatal("expected POST /webhooks")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode POST: %v", err)
	}
	if got := payload["endpointID"]; got != float64(1) {
		t.Errorf("payload.endpointID: expected 1, got %v", got)
	}
	if got := payload["resourceID"]; got != "abc123" {
		t.Errorf("payload.resourceID: expected %q, got %v", "abc123", got)
	}
	if got := payload["webhookType"]; got != float64(1) {
		t.Errorf("payload.webhookType: expected 1, got %v", got)
	}
}

// TestWebhookCreate_HTTPError verifies that a 5xx response propagates.
func TestWebhookCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/webhooks", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"boom"}`,
	))

	r := resourceWebhook()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("resource_id", "abc")
	_ = d.Set("webhook_type", 1)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestWebhookRead_NoOp verifies that Read does not hit the network — the
// resource declares Read as a no-op (Read returns nil with no work).
func TestWebhookRead_NoOp(t *testing.T) {
	mock := NewMockServer(t)
	r := resourceWebhook()
	d := r.TestResourceData()
	d.SetId("42")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if len(mock.Requests()) != 0 {
		t.Errorf("expected zero HTTP requests for no-op Read, got %d", len(mock.Requests()))
	}
}

// TestWebhookUpdate_NoChangeIsNoOp verifies that Update without any detected
// change (HasChange("registry_id") == false) does not send any HTTP request.
// Build-time TestResourceData has no diff state, so HasChange is always false
// — this confirms the guard.
func TestWebhookUpdate_NoChangeIsNoOp(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceWebhook()
	d := r.TestResourceData()
	d.SetId("9")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("resource_id", "abc")
	_ = d.Set("webhook_type", 1)
	_ = d.Set("registry_id", 4)

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if mock.FindRequest("PUT", "/webhooks/9") != nil {
		t.Error("did not expect PUT when HasChange(registry_id) is false")
	}
}

// TestWebhookDelete_HappyPath verifies the DELETE call is sent and the
// resource clears its ID afterwards. Note: the generated SDK contract treats
// 202 (not 204) as the success code for this endpoint.
func TestWebhookDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/webhooks/12", RespondString(http.StatusAccepted, "", ""))

	r := resourceWebhook()
	d := r.TestResourceData()
	d.SetId("12")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/webhooks/12") == nil {
		t.Error("expected DELETE /webhooks/12 to be sent")
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared after Delete, got %q", d.Id())
	}
}
