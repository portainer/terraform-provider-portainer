package internal

import (
	"net/http"
	"testing"
)

// TestOpenAMTDeviceActionCreate_HappyPath verifies POST to
// /open_amt/{envID}/devices/{deviceID}/action with JSON body {"action": ...}
// and sets ID "openamt-device-<deviceID>-action-<action>".
func TestOpenAMTDeviceActionCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/open_amt/5/devices/42/action", RespondString(http.StatusOK, "", ""))

	r := resourcePortainerOpenAMTDeviceAction()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 5)
	_ = d.Set("device_id", 42)
	_ = d.Set("action", "poweron")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "openamt-device-42-action-poweron" {
		t.Errorf("expected ID %q, got %q", "openamt-device-42-action-poweron", d.Id())
	}
	post := mock.FindRequest("POST", "/open_amt/5/devices/42/action")
	if post == nil {
		t.Fatal("expected POST recorded")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["action"] != "poweron" {
		t.Errorf("payload.action: expected poweron, got %v", payload["action"])
	}
	if ct := post.Headers.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type: expected application/json, got %q", ct)
	}
}

// TestOpenAMTDeviceActionCreate_HTTPError verifies HTTP error propagates.
func TestOpenAMTDeviceActionCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/open_amt/5/devices/42/action",
		RespondString(http.StatusBadGateway, "application/json", `{"message":"AMT host unreachable"}`))

	r := resourcePortainerOpenAMTDeviceAction()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 5)
	_ = d.Set("device_id", 42)
	_ = d.Set("action", "reset")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 502, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}
