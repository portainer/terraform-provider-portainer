package internal

import (
	"net/http"
	"testing"
)

// These tests cover the Settings Apply/Read branches the base + cov3 suites
// skip: the internal_auth_settings, global_deployment_options and
// black_listed_labels payload builders, the oauth kube_secret_key branch, and
// the Read-side flatten of internal_auth_settings + global_deployment_options.

// TestSettingsCov2_Apply_ExtraBlocks flips the remaining conditional payload
// builders so their branches run and serialize with the expected JSON keys.
func TestSettingsCov2_Apply_ExtraBlocks(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/settings", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceSettings()
	d := r.TestResourceData()
	_ = d.Set("authentication_method", 3)
	_ = d.Set("internal_auth_settings", []interface{}{
		map[string]interface{}{"required_password_length": 12},
	})
	_ = d.Set("global_deployment_options", []interface{}{
		map[string]interface{}{"hide_stacks_functionality": true},
	})
	_ = d.Set("black_listed_labels", []interface{}{
		map[string]interface{}{"name": "secret", "value": "hidden"},
	})
	_ = d.Set("oauth_settings", []interface{}{
		map[string]interface{}{
			"client_id":       "client-x",
			"kube_secret_key": []interface{}{1, 2, 3},
		},
	})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/settings")
	if put == nil {
		t.Fatal("expected a PUT to /settings")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode PUT body: %v", err)
	}

	ias, ok := payload["internalAuthSettings"].(map[string]interface{})
	if !ok {
		t.Fatalf("internalAuthSettings: expected object, got %T", payload["internalAuthSettings"])
	}
	if got := ias["requiredPasswordLength"]; got != float64(12) {
		t.Errorf("internalAuthSettings.requiredPasswordLength: expected 12, got %v", got)
	}

	gdo, ok := payload["globalDeploymentOptions"].(map[string]interface{})
	if !ok {
		t.Fatalf("globalDeploymentOptions: expected object, got %T", payload["globalDeploymentOptions"])
	}
	if got := gdo["hideStacksFunctionality"]; got != true {
		t.Errorf("globalDeploymentOptions.hideStacksFunctionality: expected true, got %v", got)
	}

	labels, ok := payload["blackListedLabels"].([]interface{})
	if !ok || len(labels) != 1 {
		t.Fatalf("blackListedLabels: expected 1 entry, got %v", payload["blackListedLabels"])
	}
	label := labels[0].(map[string]interface{})
	if label["name"] != "secret" || label["value"] != "hidden" {
		t.Errorf("blackListedLabels[0]: got %v", label)
	}

	oauth, ok := payload["oauthSettings"].(map[string]interface{})
	if !ok {
		t.Fatalf("oauthSettings: expected object, got %T", payload["oauthSettings"])
	}
	ksk, ok := oauth["KubeSecretKey"].([]interface{})
	if !ok || len(ksk) != 3 {
		t.Errorf("oauthSettings.KubeSecretKey: expected 3 entries, got %v", oauth["KubeSecretKey"])
	}
}

// TestSettingsCov2_Read_InternalAndGlobalBlocks covers the Read flatten of the
// internal_auth_settings and global_deployment_options blocks.
func TestSettingsCov2_Read_InternalAndGlobalBlocks(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/settings", RespondJSON(http.StatusOK, map[string]interface{}{
		"authenticationMethod": 1,
		"internalAuthSettings": map[string]interface{}{
			"requiredPasswordLength": 14,
		},
		"globalDeploymentOptions": map[string]interface{}{
			"hideStacksFunctionality": true,
		},
	}))

	r := resourceSettings()
	d := r.TestResourceData()
	d.SetId("portainer-settings")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	iasList := d.Get("internal_auth_settings").([]interface{})
	if len(iasList) != 1 {
		t.Fatalf("internal_auth_settings: expected 1 block, got %d", len(iasList))
	}
	if got := iasList[0].(map[string]interface{})["required_password_length"]; got != 14 {
		t.Errorf("required_password_length: expected 14, got %v", got)
	}

	gdoList := d.Get("global_deployment_options").([]interface{})
	if len(gdoList) != 1 {
		t.Fatalf("global_deployment_options: expected 1 block, got %d", len(gdoList))
	}
	if got := gdoList[0].(map[string]interface{})["hide_stacks_functionality"]; got != true {
		t.Errorf("hide_stacks_functionality: expected true, got %v", got)
	}
}
