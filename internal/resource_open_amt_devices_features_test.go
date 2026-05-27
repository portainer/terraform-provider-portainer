package internal

import (
	"net/http"
	"testing"
)

// TestOpenAMTDevicesFeaturesCreate_HappyPath verifies POST to
// /open_amt/{envID}/devices_features/{deviceID} carries a nested
// {"features": {...}} body and sets ID "amt-device-features-<deviceID>".
func TestOpenAMTDevicesFeaturesCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/open_amt/5/devices_features/42", RespondString(http.StatusOK, "", ""))

	r := resourcePortainerOpenAMTDevicesFeatures()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 5)
	_ = d.Set("device_id", 42)
	_ = d.Set("ider", true)
	_ = d.Set("kvm", true)
	_ = d.Set("sol", false)
	_ = d.Set("redirection", true)
	_ = d.Set("user_consent", "kvmOnly")

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "amt-device-features-42" {
		t.Errorf("expected ID %q, got %q", "amt-device-features-42", d.Id())
	}
	post := mock.FindRequest("POST", "/open_amt/5/devices_features/42")
	if post == nil {
		t.Fatal("expected POST recorded")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	features, ok := payload["features"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected features map, got %T", payload["features"])
	}
	if features["IDER"] != true {
		t.Errorf("features.IDER: expected true, got %v", features["IDER"])
	}
	if features["KVM"] != true {
		t.Errorf("features.KVM: expected true, got %v", features["KVM"])
	}
	if features["SOL"] != false {
		t.Errorf("features.SOL: expected false, got %v", features["SOL"])
	}
	if features["redirection"] != true {
		t.Errorf("features.redirection: expected true, got %v", features["redirection"])
	}
	if features["userConsent"] != "kvmOnly" {
		t.Errorf("features.userConsent: expected kvmOnly, got %v", features["userConsent"])
	}
}

// TestOpenAMTDevicesFeaturesCreate_HTTPError verifies HTTP error propagates.
func TestOpenAMTDevicesFeaturesCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/open_amt/5/devices_features/42",
		RespondString(http.StatusForbidden, "application/json", `{"message":"forbidden"}`))

	r := resourcePortainerOpenAMTDevicesFeatures()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 5)
	_ = d.Set("device_id", 42)
	_ = d.Set("user_consent", "none")

	if err := r.Create(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 403, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}
