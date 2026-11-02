package internal

import (
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// TestEndpointSettingsUpdate_SendsSecuritySwitchesTopLevel verifies that the
// security switches reach PUT /endpoints/{id}/settings as top-level fields,
// which is where Portainer CE reads them, including the ones set to false.
func TestEndpointSettingsUpdate_SendsSecuritySwitchesTopLevel(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/endpoints/7/settings", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/endpoints/7", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 7}))

	r := resourceEndpointSettings()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 7)
	_ = d.Set("security_settings", []interface{}{
		map[string]interface{}{
			"allow_bind_mounts":            false,
			"allow_container_capabilities": false,
			"allow_device_mapping":         false,
			"allow_host_namespace":         false,
			"allow_privileged_mode":        false,
			"allow_stack_management":       false,
			"allow_sysctl_setting":         false,
			"allow_volume_browser":         true,
			"enable_host_management":       false,
		},
	})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/endpoints/7/settings")
	if put == nil {
		t.Fatal("expected PUT /endpoints/7/settings")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode PUT body: %v", err)
	}

	want := map[string]bool{
		"allowBindMountsForRegularUsers":            false,
		"allowContainerCapabilitiesForRegularUsers": false,
		"allowDeviceMappingForRegularUsers":         false,
		"allowHostNamespaceForRegularUsers":         false,
		"allowPrivilegedModeForRegularUsers":        false,
		"allowStackManagementForRegularUsers":       false,
		"allowSysctlSettingForRegularUsers":         false,
		"allowVolumeBrowserForRegularUsers":         true,
		"enableHostManagementFeatures":              false,
	}
	for key, value := range want {
		got, ok := payload[key]
		if !ok {
			t.Errorf("%s missing from the top level of the payload", key)
			continue
		}
		if got != value {
			t.Errorf("%s: expected %v, got %v", key, value, got)
		}
	}
}

// TestEndpointSettingsUpdate_NoSecurityBlockSendsNoSwitches verifies that
// without a security_settings block no switch is sent, so Portainer keeps the
// environment's current values.
func TestEndpointSettingsUpdate_NoSecurityBlockSendsNoSwitches(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/endpoints/7/settings", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/endpoints/7", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 7}))

	r := resourceEndpointSettings()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 7)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var payload map[string]interface{}
	if err := mock.FindRequest("PUT", "/endpoints/7/settings").DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode PUT body: %v", err)
	}
	if _, ok := payload["allowStackManagementForRegularUsers"]; ok {
		t.Errorf("expected no security switches without a security_settings block, got %v", payload)
	}
}

// TestEndpointSettingsUpdate_GPUList: GPUs removed from the configuration have
// to be cleared in Portainer (an empty list), while GPUs the configuration
// does not touch are left alone (null). Otherwise an imported environment with
// GPUs planned their removal on every run.
func TestEndpointSettingsUpdate_GPUList(t *testing.T) {
	state := &terraform.InstanceState{
		ID: "7",
		Attributes: map[string]string{
			"id": "7", "endpoint_id": "7",
			"gpus.#": "1", "gpus.0.name": "nvidia", "gpus.0.value": "gpu0",
		},
	}
	unmanaged := &terraform.InstanceState{ID: "7", Attributes: map[string]string{"id": "7", "endpoint_id": "7"}}
	for name, tc := range map[string]struct {
		state *terraform.InstanceState
		diff  *terraform.InstanceDiff
		want  string
	}{
		"removed": {state, &terraform.InstanceDiff{Attributes: map[string]*terraform.ResourceAttrDiff{
			"gpus.#": {Old: "1", New: "0"}, "gpus.0.name": {Old: "nvidia", NewRemoved: true}, "gpus.0.value": {Old: "gpu0", NewRemoved: true},
		}}, "[]"},
		"managed and unchanged": {state, &terraform.InstanceDiff{Attributes: map[string]*terraform.ResourceAttrDiff{
			"enable_gpu_management": {Old: "false", New: "true"},
		}}, "sent"},
		"not managed": {unmanaged, &terraform.InstanceDiff{Attributes: map[string]*terraform.ResourceAttrDiff{
			"enable_gpu_management": {Old: "false", New: "true"},
		}}, "null"},
	} {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			mock.On("PUT", "/endpoints/7/settings", RespondJSON(http.StatusOK, map[string]interface{}{}))
			mock.On("GET", "/endpoints/7", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 7}))
			r := resourceEndpointSettings()
			d, err := schema.InternalMap(r.SchemaMap()).Data(tc.state, tc.diff)
			if err != nil {
				t.Fatalf("Data failed: %v", err)
			}
			if err := rcUpdate(r, d, mock.Client()); err != nil {
				t.Fatalf("Update failed: %v", err)
			}
			var payload map[string]interface{}
			if err := mock.FindRequest("PUT", "/endpoints/7/settings").DecodeJSON(&payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			gpus, present := payload["gpus"]
			switch tc.want {
			case "[]":
				if list, ok := gpus.([]interface{}); !ok || len(list) != 0 {
					t.Errorf("gpus: expected an empty list to clear them, got %v", gpus)
				}
			case "sent":
				if list, ok := gpus.([]interface{}); !ok || len(list) != 1 {
					t.Errorf("gpus: expected the managed list, got %v", gpus)
				}
			default:
				if !present || gpus != nil {
					t.Errorf("gpus: expected null so Portainer keeps them, got %v (present=%v)", gpus, present)
				}
			}
		})
	}
}
