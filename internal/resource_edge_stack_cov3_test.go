package internal

import (
	"net/http"
	"testing"
)

// TestEdgeStackCov3_Read_AutoUpdateWebhookAndGitAuth covers the richer Read
// branches: GitConfig with Authentication, AutoUpdate carrying a webhook,
// SupportRelativePath, and EnvVars mapping.
func TestEdgeStackCov3_Read_AutoUpdateWebhookAndGitAuth(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/edge_stacks/9", RespondJSON(http.StatusOK, map[string]interface{}{
		"Name":                "gitops-stack",
		"DeploymentType":      0,
		"EdgeGroups":          []int{1, 2},
		"Registries":          []int{3},
		"SupportRelativePath": true,
		"FilesystemPath":      "/data",
		"RePullImage":         true,
		"EnvVars": []map[string]interface{}{
			{"name": "FOO", "value": "bar"},
		},
		"GitConfig": map[string]interface{}{
			"URL":            "https://git.example.com/repo.git",
			"ReferenceName":  "refs/heads/main",
			"ConfigFilePath": "docker-compose.yml",
			"Authentication": map[string]interface{}{
				"Username":        "gituser",
				"GitCredentialID": 4,
			},
		},
		"AutoUpdate": map[string]interface{}{
			"Interval":       "5m",
			"Webhook":        "wh-uuid-123",
			"ForcePullImage": true,
			"ForceUpdate":    true,
		},
	}))

	r := resourceEdgeStack()
	d := r.TestResourceData()
	d.SetId("9")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Get("relative_path"); got != "/data" {
		t.Errorf("relative_path: got %v", got)
	}
	if got := d.Get("git_repository_authentication"); got != true {
		t.Errorf("git_repository_authentication: expected true, got %v", got)
	}
	if got := d.Get("repository_username"); got != "gituser" {
		t.Errorf("repository_username: got %v", got)
	}
	if got := d.Get("stack_webhook"); got != true {
		t.Errorf("stack_webhook: expected true, got %v", got)
	}
	if got := d.Get("webhook_id"); got != "wh-uuid-123" {
		t.Errorf("webhook_id: got %v", got)
	}
	if got := d.Get("update_interval"); got != "5m" {
		t.Errorf("update_interval: got %v", got)
	}
	env := d.Get("environment").(map[string]interface{})
	if env["FOO"] != "bar" {
		t.Errorf("environment[FOO]: got %v", env["FOO"])
	}
}

// TestEdgeStackCov3_Create_FilePathOpenError covers the os.Open failure branch
// in the stack_file_path creation method. The duplicate-detection list runs
// first, then opening a non-existent file must surface as an error.
func TestEdgeStackCov3_Create_FilePathOpenError(t *testing.T) {
	mock := NewMockServer(t)

	// findExistingEdgeStackByName lists stacks first; return an empty list so
	// Create proceeds to the file-path branch.
	mock.On("GET", "/edge_stacks", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	r := resourceEdgeStack()
	d := r.TestResourceData()
	_ = d.Set("name", "from-file")
	_ = d.Set("deployment_type", 0)
	_ = d.Set("edge_groups", []interface{}{1})
	_ = d.Set("stack_file_path", "/nonexistent/path/does-not-exist.yml")

	err := rcCreate(r, d, mock.Client())
	if err == nil {
		t.Fatal("expected error opening non-existent stack file, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}
