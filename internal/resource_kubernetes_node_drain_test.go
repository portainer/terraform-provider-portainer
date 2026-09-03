package internal

import (
	"net/http"
	"testing"
)

// TestKubernetesNodeDrain_SendsOptions verifies the drain request lands on the
// 2.45 route and carries every advanced option under the field names
// Portainer's untagged drainNodePayload expects.
func TestKubernetesNodeDrain_SendsOptions(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/kubernetes/4/nodes/worker-1/drain", RespondString(http.StatusNoContent, "", ""))

	r := resourceKubernetesNodeDrain()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 4)
	_ = d.Set("node_name", "worker-1")
	_ = d.Set("force", true)
	_ = d.Set("timeout_seconds", 300)
	_ = d.Set("grace_period_seconds", 30)
	_ = d.Set("ignore_daemon_sets", true)
	_ = d.Set("delete_empty_dir_data", false)
	_ = d.Set("disable_eviction", true)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if got := d.Id(); got != "4/worker-1" {
		t.Errorf("id: expected %q, got %q", "4/worker-1", got)
	}

	post := mock.FindRequest("POST", "/kubernetes/4/nodes/worker-1/drain")
	if post == nil {
		t.Fatal("expected POST to the drain endpoint")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	for key, want := range map[string]interface{}{
		"Force":              true,
		"TimeoutSeconds":     float64(300),
		"GracePeriodSeconds": float64(30),
		"IgnoreDaemonSets":   true,
		"DeleteEmptyDirData": false,
		"DisableEviction":    true,
	} {
		if got := payload[key]; got != want {
			t.Errorf("payload.%s: expected %v, got %v", key, want, got)
		}
	}
}

// TestKubernetesNodeDrain_DefaultsMatchPortainer guards the schema defaults
// against Portainer's libkubectl.DefaultDrainOptions. They are sent explicitly,
// so a drift here would silently change what a bare drain does.
func TestKubernetesNodeDrain_DefaultsMatchPortainer(t *testing.T) {
	r := resourceKubernetesNodeDrain()

	for field, want := range map[string]interface{}{
		"force":                 false,
		"timeout_seconds":       60,
		"grace_period_seconds":  -1,
		"ignore_daemon_sets":    true,
		"delete_empty_dir_data": true,
		"disable_eviction":      false,
	} {
		s, ok := r.Schema[field]
		if !ok {
			t.Errorf("%s: field missing from the schema", field)
			continue
		}
		if s.Default != want {
			t.Errorf("%s: expected default %v, got %v", field, want, s.Default)
		}
	}
}

// TestKubernetesNodeDrain_APIError verifies a failing drain surfaces as an
// error instead of a silently successful apply.
func TestKubernetesNodeDrain_APIError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/kubernetes/4/nodes/worker-1/drain",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"failed to drain node"}`))

	r := resourceKubernetesNodeDrain()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 4)
	_ = d.Set("node_name", "worker-1")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected Create to fail when the drain request errors")
	}
	if d.Id() != "" {
		t.Errorf("id should stay empty on failure, got %q", d.Id())
	}
}
