package internal

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestExperimentalSettingsCreate_HappyPath verifies that the apply action
// PUTs to /settings/experimental with openAIIntegration and sets a stable ID.
func TestExperimentalSettingsCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/settings/experimental", RespondString(http.StatusOK, "application/json", `{}`))

	r := resourceExperimentalSettings()
	d := r.TestResourceData()
	_ = d.Set("openai_integration", true)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "portainer-experimental-settings" {
		t.Errorf("ID: got %q", d.Id())
	}

	req := mock.FindRequest("PUT", "/settings/experimental")
	if req == nil {
		t.Fatal("expected PUT /settings/experimental")
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(req.Body, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if got := payload["openAIIntegration"]; got != true {
		t.Errorf("openAIIntegration: got %v", got)
	}
}

// TestExperimentalSettingsRead_HappyPath verifies that Read decodes the
// experimentalFeatures envelope and populates state.
func TestExperimentalSettingsRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/settings/experimental", RespondJSON(http.StatusOK, map[string]interface{}{
		"experimentalFeatures": map[string]interface{}{
			"OpenAIIntegration": true,
		},
	}))

	r := resourceExperimentalSettings()
	d := r.TestResourceData()
	d.SetId("portainer-experimental-settings")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Get("openai_integration"); got != true {
		t.Errorf("openai_integration: got %v", got)
	}
	if d.Id() != "portainer-experimental-settings" {
		t.Errorf("expected stable ID, got %q", d.Id())
	}
}

// TestExperimentalSettingsUpdate_HappyPath verifies the Update path (which
// reuses the apply handler) sends a PUT.
func TestExperimentalSettingsUpdate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/settings/experimental", RespondString(http.StatusOK, "application/json", `{}`))

	r := resourceExperimentalSettings()
	d := r.TestResourceData()
	d.SetId("portainer-experimental-settings")
	_ = d.Set("openai_integration", false)

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	req := mock.FindRequest("PUT", "/settings/experimental")
	if req == nil {
		t.Fatal("expected PUT /settings/experimental on Update")
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(req.Body, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if got := payload["openAIIntegration"]; got != false {
		t.Errorf("openAIIntegration: got %v", got)
	}
}

// TestExperimentalSettingsDelete_ClearsID verifies Delete only clears state
// (no DELETE endpoint).
func TestExperimentalSettingsDelete_ClearsID(t *testing.T) {
	r := resourceExperimentalSettings()
	d := r.TestResourceData()
	d.SetId("portainer-experimental-settings")

	if err := rcDelete(r, d, nil); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestExperimentalSettingsCreate_HTTPError verifies error propagation.
func TestExperimentalSettingsCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/settings/experimental", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"oops"}`,
	))

	r := resourceExperimentalSettings()
	d := r.TestResourceData()
	_ = d.Set("openai_integration", true)

	err := rcCreate(r, d, mock.Client())
	if err == nil {
		t.Fatal("expected error on 500, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}
