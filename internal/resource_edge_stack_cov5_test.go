package internal

import (
	"net/http"
	"testing"
)

// =========================================================================
// cov5 coverage for resource_edge_stack.go: the autoUpdate-without-webhook
// branch (update_interval set but stack_webhook false) in both the repository
// Create and git Update paths, and the file/content Update path triggered by
// stack_file_path rather than stack_file_content.
// =========================================================================

// TestEdgeStackCov5_Create_Repository_IntervalNoWebhook covers the repository
// create branch where update_interval is set but stack_webhook is false: the
// autoUpdate block is still built, but with an empty webhook, so the
// `if webhookID != ""` computed-output branch is NOT taken.
func TestEdgeStackCov5_Create_Repository_IntervalNoWebhook(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/edge_stacks", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/edge_stacks/create/repository", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 91, "Name": "git-interval",
	}))
	mock.On("GET", "/edge_stacks/91", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   91,
		"Name": "git-interval",
		// AutoUpdate present but no webhook: Read must land in the
		// webhook-cleared branch.
		"AutoUpdate": map[string]interface{}{
			"Interval": "10m",
		},
	}))

	r := resourceEdgeStack()
	d := r.TestResourceData()
	_ = d.Set("name", "git-interval")
	_ = d.Set("deployment_type", 0)
	_ = d.Set("edge_groups", []interface{}{4})
	_ = d.Set("repository_url", "https://github.com/example/repo.git")
	_ = d.Set("repository_reference_name", "refs/heads/main")
	_ = d.Set("update_interval", "10m")
	// stack_webhook intentionally left false.

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "91" {
		t.Errorf("expected ID 91, got %q", d.Id())
	}

	post := mock.FindRequest("POST", "/edge_stacks/create/repository")
	if post == nil {
		t.Fatal("expected POST /edge_stacks/create/repository")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	autoUpdate, ok := payload["autoUpdate"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected autoUpdate block, got %v", payload["autoUpdate"])
	}
	if autoUpdate["interval"] != "10m" {
		t.Errorf("autoUpdate.interval: expected 10m, got %v", autoUpdate["interval"])
	}
	if autoUpdate["webhook"] != "" {
		t.Errorf("expected empty webhook when stack_webhook is false, got %v", autoUpdate["webhook"])
	}
	// No webhook computed since stack_webhook was false.
	if got := d.Get("webhook_id").(string); got != "" {
		t.Errorf("expected empty webhook_id, got %q", got)
	}
}

// TestEdgeStackCov5_Update_FilePath_NoContent covers the file/content Update
// branch entered via stack_file_path (rather than stack_file_content): the
// branch runs, but the `if stack_file_content` sub-branch is skipped so
// stackFileContent is omitted from the PUT body.
func TestEdgeStackCov5_Update_FilePath_NoContent(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/edge_stacks/92", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 92, "Name": "file-upd",
	}))
	mock.On("GET", "/edge_stacks/92", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 92, "Name": "file-upd",
	}))

	r := resourceEdgeStack()
	d := r.TestResourceData()
	d.SetId("92")
	_ = d.Set("name", "file-upd")
	_ = d.Set("deployment_type", 0)
	_ = d.Set("edge_groups", []interface{}{1})
	_ = d.Set("registries", []interface{}{})
	// Only stack_file_path is set (no stack_file_content).
	_ = d.Set("stack_file_path", "/some/path/docker-compose.yml")

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/edge_stacks/92")
	if put == nil {
		t.Fatal("expected PUT /edge_stacks/92")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode PUT body: %v", err)
	}
	if _, present := payload["stackFileContent"]; present {
		t.Errorf("expected stackFileContent to be OMITTED when only stack_file_path is set, got %v", payload["stackFileContent"])
	}
	if got := payload["updateVersion"]; got != true {
		t.Errorf("payload.updateVersion: expected true, got %v", got)
	}
}

// TestEdgeStackCov5_Update_Repository_IntervalNoWebhook covers the git Update
// branch where update_interval is set but stack_webhook is false: autoUpdate is
// built with an empty webhook, skipping the computed-output branch.
func TestEdgeStackCov5_Update_Repository_IntervalNoWebhook(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/edge_stacks/93/git", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 93,
	}))
	mock.On("GET", "/edge_stacks/93", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   93,
		"Name": "git-int-upd",
		"AutoUpdate": map[string]interface{}{
			"Interval": "15m",
		},
	}))

	r := resourceEdgeStack()
	d := r.TestResourceData()
	d.SetId("93")
	_ = d.Set("name", "git-int-upd")
	_ = d.Set("deployment_type", 0)
	_ = d.Set("edge_groups", []interface{}{9})
	_ = d.Set("registries", []interface{}{})
	_ = d.Set("repository_url", "https://github.com/example/repo.git")
	_ = d.Set("repository_reference_name", "refs/heads/main")
	_ = d.Set("update_interval", "15m")
	// stack_webhook intentionally left false.

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/edge_stacks/93/git")
	if put == nil {
		t.Fatal("expected PUT /edge_stacks/93/git")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode PUT body: %v", err)
	}
	autoUpdate, ok := payload["autoUpdate"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected autoUpdate block, got %v", payload["autoUpdate"])
	}
	if autoUpdate["interval"] != "15m" {
		t.Errorf("autoUpdate.interval: expected 15m, got %v", autoUpdate["interval"])
	}
	if got := d.Get("webhook_id").(string); got != "" {
		t.Errorf("expected empty webhook_id when stack_webhook is false, got %q", got)
	}
}
