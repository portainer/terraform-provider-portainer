package internal

import (
	"net/http"
	"testing"
)

// TestEndpointSettingsCreate_HappyPath verifies that creating the resource
// sends a PUT to /endpoints/{id}/settings with the expected payload shape
// (camelCase top-level keys, nested security/changeWindow/deployment objects)
// and sets the ID to the endpoint_id.
func TestEndpointSettingsCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/endpoints/7/settings", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceEndpointSettings()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 7)
	_ = d.Set("enable_gpu_management", true)
	_ = d.Set("enable_image_notification", true)
	_ = d.Set("gpus", []interface{}{
		map[string]interface{}{"name": "gpu0", "value": "nvidia-0"},
	})
	_ = d.Set("change_window", []interface{}{
		map[string]interface{}{"enabled": true, "start_time": "01:00", "end_time": "03:00"},
	})
	_ = d.Set("deployment_options", []interface{}{
		map[string]interface{}{
			"hide_add_with_form":      true,
			"hide_file_upload":        false,
			"hide_web_editor":         true,
			"override_global_options": true,
		},
	})
	_ = d.Set("security_settings", []interface{}{
		map[string]interface{}{
			"allow_bind_mounts":            true,
			"allow_container_capabilities": false,
			"allow_device_mapping":         true,
			"allow_host_namespace":         false,
			"allow_privileged_mode":        true,
			"allow_stack_management":       true,
			"allow_sysctl_setting":         false,
			"allow_volume_browser":         true,
			"enable_host_management":       true,
		},
	})

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "7" {
		t.Errorf("expected ID %q, got %q", "7", d.Id())
	}

	put := mock.FindRequest("PUT", "/endpoints/7/settings")
	if put == nil {
		t.Fatal("expected PUT /endpoints/7/settings")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode PUT body: %v", err)
	}
	if payload["enableGPUManagement"] != true {
		t.Errorf("enableGPUManagement: expected true, got %v", payload["enableGPUManagement"])
	}
	if payload["enableImageNotification"] != true {
		t.Errorf("enableImageNotification: expected true, got %v", payload["enableImageNotification"])
	}
	gpus, ok := payload["gpus"].([]interface{})
	if !ok || len(gpus) != 1 {
		t.Fatalf("expected gpus to be a list of 1, got %v", payload["gpus"])
	}
	gpu0 := gpus[0].(map[string]interface{})
	if gpu0["name"] != "gpu0" || gpu0["value"] != "nvidia-0" {
		t.Errorf("gpus[0]: expected name=gpu0/value=nvidia-0, got %v", gpu0)
	}
	cw, ok := payload["changeWindow"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected changeWindow map, got %v", payload["changeWindow"])
	}
	if cw["Enabled"] != true || cw["StartTime"] != "01:00" || cw["EndTime"] != "03:00" {
		t.Errorf("changeWindow: unexpected payload %v", cw)
	}
	sec, ok := payload["securitySettings"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected securitySettings map, got %v", payload["securitySettings"])
	}
	if sec["allowBindMountsForRegularUsers"] != true {
		t.Errorf("securitySettings.allowBindMountsForRegularUsers: expected true, got %v", sec["allowBindMountsForRegularUsers"])
	}
	if sec["enableHostManagementFeatures"] != true {
		t.Errorf("securitySettings.enableHostManagementFeatures: expected true, got %v", sec["enableHostManagementFeatures"])
	}
}

// TestEndpointSettingsCreate_Minimal verifies the resource works with only
// the required endpoint_id and the boolean defaults — i.e. no nested blocks.
func TestEndpointSettingsCreate_Minimal(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/endpoints/1/settings", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceEndpointSettings()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "1" {
		t.Errorf("expected ID %q, got %q", "1", d.Id())
	}

	put := mock.FindRequest("PUT", "/endpoints/1/settings")
	if put == nil {
		t.Fatal("expected PUT /endpoints/1/settings")
	}
	var payload map[string]interface{}
	_ = put.DecodeJSON(&payload)
	// Defaults: enableGPUManagement is required field, always present.
	if payload["enableGPUManagement"] != false {
		t.Errorf("enableGPUManagement default: expected false, got %v", payload["enableGPUManagement"])
	}
}

// TestEndpointSettingsUpdate_HappyPath verifies the Update path (same handler
// as Create) re-sends PUT with new values.
func TestEndpointSettingsUpdate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/endpoints/3/settings", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceEndpointSettings()
	d := r.TestResourceData()
	d.SetId("3")
	_ = d.Set("endpoint_id", 3)
	_ = d.Set("enable_gpu_management", true)

	if err := r.Update(d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	put := mock.FindRequest("PUT", "/endpoints/3/settings")
	if put == nil {
		t.Fatal("expected PUT /endpoints/3/settings")
	}
	var payload map[string]interface{}
	_ = put.DecodeJSON(&payload)
	if payload["enableGPUManagement"] != true {
		t.Errorf("enableGPUManagement: expected true, got %v", payload["enableGPUManagement"])
	}
}

// TestEndpointSettingsRead_SetsID verifies the no-op Read still pins the ID
// to endpoint_id.
func TestEndpointSettingsRead_SetsID(t *testing.T) {
	r := resourceEndpointSettings()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 42)

	if err := r.Read(d, nil); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "42" {
		t.Errorf("expected ID %q, got %q", "42", d.Id())
	}
}

// TestEndpointSettingsDelete_ClearsID verifies Delete is a state-only no-op.
func TestEndpointSettingsDelete_ClearsID(t *testing.T) {
	r := resourceEndpointSettings()
	d := r.TestResourceData()
	d.SetId("42")

	if err := r.Delete(d, nil); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestEndpointSettingsCreate_HTTPError verifies that a non-200 surfaces an
// error and leaves the ID empty.
func TestEndpointSettingsCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/endpoints/9/settings", RespondString(
		http.StatusBadRequest, "application/json",
		`{"message":"bad request"}`,
	))

	r := resourceEndpointSettings()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 9)

	if err := r.Create(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}
