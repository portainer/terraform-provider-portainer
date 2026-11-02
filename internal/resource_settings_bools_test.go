package internal

import (
	"net/http"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// TestSettingsApply_SendsConfiguredFalse is the regression test for switches
// that could be turned on but never off: the payload tagged them omitempty, so
// false was dropped and Portainer kept the setting enabled. A switch the
// configuration leaves out must still not be sent, or Portainer's own value
// would be overwritten.
func TestSettingsApply_SendsConfiguredFalse(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/settings", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceSettings()
	d := r.Data(&terraform.InstanceState{
		ID: "portainer-settings",
		Attributes: map[string]string{
			"id":                           "portainer-settings",
			"enable_edge_compute_features": "false",
			"disable_kube_shell":           "false",
			"trust_on_first_connect":       "true",
		},
		RawConfig: cty.ObjectVal(map[string]cty.Value{
			"enable_edge_compute_features": cty.False,
			"disable_kube_shell":           cty.False,
			"trust_on_first_connect":       cty.NullVal(cty.Bool),
		}),
	})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	var payload map[string]interface{}
	if err := mock.FindRequest("PUT", "/settings").DecodeJSON(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	for _, key := range []string{"enableEdgeComputeFeatures", "DisableKubeShell"} {
		if got, ok := payload[key]; !ok || got != false {
			t.Errorf("payload.%s: expected the configured false, got %v (present=%v)", key, got, ok)
		}
	}
	if got, ok := payload["trustOnFirstConnect"]; ok {
		t.Errorf("payload.trustOnFirstConnect: expected it left out when not configured, got %v", got)
	}
	if _, ok := payload["enforceEdgeID"]; ok {
		t.Error("payload.enforceEdgeID: expected it left out when not configured")
	}
}

func TestSettingsApply_SendsForceSecureCookies(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/settings", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/settings", RespondJSON(http.StatusOK, map[string]interface{}{"ForceSecureCookies": true}))

	r := resourceSettings()
	d := r.Data(&terraform.InstanceState{
		ID:         "portainer-settings",
		Attributes: map[string]string{"id": "portainer-settings", "force_secure_cookies": "true"},
		RawConfig:  cty.ObjectVal(map[string]cty.Value{"force_secure_cookies": cty.True}),
	})
	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	var payload map[string]interface{}
	if err := mock.FindRequest("PUT", "/settings").DecodeJSON(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload["ForceSecureCookies"] != true {
		t.Errorf("payload.ForceSecureCookies: expected true, got %v", payload["ForceSecureCookies"])
	}

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if !d.Get("force_secure_cookies").(bool) {
		t.Error("force_secure_cookies: expected true read back")
	}
}
