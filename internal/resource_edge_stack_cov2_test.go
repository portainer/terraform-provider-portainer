package internal

import (
	"context"
	"net/http"
	"testing"
)

// TestEdgeStackCov2_ToJSONString covers the toJSONString helper.
func TestEdgeStackCov2_ToJSONString(t *testing.T) {
	if got := toJSONString([]int{1, 2, 3}); got != "[1,2,3]" {
		t.Errorf("toJSONString([]int{1,2,3}) = %q, want %q", got, "[1,2,3]")
	}
	if got := toJSONString([]int{}); got != "[]" {
		t.Errorf("toJSONString([]int{}) = %q, want %q", got, "[]")
	}
}

// TestEdgeStackCov2_BuildEnvVars covers buildEnvVars with and without an
// environment map.
func TestEdgeStackCov2_BuildEnvVars(t *testing.T) {
	t.Run("with env", func(t *testing.T) {
		r := resourceEdgeStack()
		d := r.TestResourceData()
		_ = d.Set("environment", map[string]interface{}{"K": "V"})
		ev := buildEnvVars(d)
		if len(ev) != 1 || ev[0]["name"] != "K" || ev[0]["value"] != "V" {
			t.Errorf("unexpected envVars: %+v", ev)
		}
	})
	t.Run("without env", func(t *testing.T) {
		r := resourceEdgeStack()
		d := r.TestResourceData()
		ev := buildEnvVars(d)
		if len(ev) != 0 {
			t.Errorf("expected empty envVars, got %+v", ev)
		}
	})
}

// TestEdgeStackCov2_SetAuthHeaders covers both header branches of setAuthHeaders.
func TestEdgeStackCov2_SetAuthHeaders(t *testing.T) {
	t.Run("api key", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "http://example/x", nil)
		setAuthHeaders(&APIClient{APIKey: "k"}, req)
		if req.Header.Get("X-API-Key") != "k" {
			t.Errorf("expected X-API-Key header, got %q", req.Header.Get("X-API-Key"))
		}
	})
	t.Run("jwt", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "http://example/x", nil)
		setAuthHeaders(&APIClient{JWTToken: "tok"}, req)
		if req.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("expected Bearer token, got %q", req.Header.Get("Authorization"))
		}
	})
}

// TestEdgeStackCov2_FindExistingByName_ListError covers the non-200 list branch
// of findExistingEdgeStackByName.
func TestEdgeStackCov2_FindExistingByName_ListError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_stacks", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	if _, err := findExistingEdgeStackByName(context.Background(), mock.Client(), "x"); err == nil {
		t.Fatal("expected error from non-200 list, got nil")
	}
}

// TestEdgeStackCov2_FindExistingByName_Match covers a successful name match.
func TestEdgeStackCov2_FindExistingByName_Match(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_stacks", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 12, "Name": "web"},
	}))

	id, err := findExistingEdgeStackByName(context.Background(), mock.Client(), "web")
	if err != nil {
		t.Fatalf("findExistingEdgeStackByName: %v", err)
	}
	if id != 12 {
		t.Errorf("expected 12, got %d", id)
	}
}

// TestEdgeStackCov2_Create_NoSourceError covers the final error branch when none
// of stack_file_content / stack_file_path / repository_url is provided.
func TestEdgeStackCov2_Create_NoSourceError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_stacks", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	r := resourceEdgeStack()
	d := r.TestResourceData()
	_ = d.Set("name", "x")
	_ = d.Set("deployment_type", 0)
	_ = d.Set("edge_groups", []interface{}{1})

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when no source provided, got nil")
	}
}

// TestEdgeStackCov2_Create_StringHTTPError covers the >=300 branch of the
// JSON create path (via createEdgeStackFromJSON).
func TestEdgeStackCov2_Create_StringHTTPError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_stacks", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/edge_stacks/create/string", RespondString(http.StatusBadRequest, "application/json", `{"message":"bad"}`))

	r := resourceEdgeStack()
	d := r.TestResourceData()
	_ = d.Set("name", "x")
	_ = d.Set("deployment_type", 0)
	_ = d.Set("edge_groups", []interface{}{1})
	_ = d.Set("stack_file_content", "version: \"3\"\n")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
}

// TestEdgeStackCov2_Create_Repository_WithWebhookAndRelPath exercises the
// repository create path including the relative_path block and the GitOps
// autoUpdate/webhook block, asserting webhook outputs get computed.
func TestEdgeStackCov2_Create_Repository_WithWebhookAndRelPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/edge_stacks", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/edge_stacks/create/repository", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   31,
		"Name": "git-wh",
	}))
	mock.On("GET", "/edge_stacks/31", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   31,
		"Name": "git-wh",
		// Readback must echo the GitOps webhook, otherwise Read drops the
		// computed webhook_id set during Create.
		"AutoUpdate": map[string]interface{}{
			"Interval": "5m",
			"Webhook":  "wh-uuid-31",
		},
	}))

	r := resourceEdgeStack()
	d := r.TestResourceData()
	_ = d.Set("name", "git-wh")
	_ = d.Set("deployment_type", 0)
	_ = d.Set("edge_groups", []interface{}{4})
	_ = d.Set("repository_url", "https://github.com/example/repo.git")
	_ = d.Set("repository_reference_name", "refs/heads/main")
	_ = d.Set("git_repository_authentication", true)
	_ = d.Set("repository_username", "bot")
	_ = d.Set("repository_password", "secret")
	_ = d.Set("relative_path", "stacks/app")
	_ = d.Set("always_clone", true)
	_ = d.Set("stack_webhook", true)
	_ = d.Set("update_interval", "5m")
	_ = d.Set("environment", map[string]interface{}{"E": "1"})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "31" {
		t.Errorf("expected ID 31, got %q", d.Id())
	}

	post := mock.FindRequest("POST", "/edge_stacks/create/repository")
	if post == nil {
		t.Fatal("expected POST /edge_stacks/create/repository")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload["supportRelativePath"] != true {
		t.Errorf("expected supportRelativePath true, got %v", payload["supportRelativePath"])
	}
	if payload["filesystemPath"] != "stacks/app" {
		t.Errorf("expected filesystemPath, got %v", payload["filesystemPath"])
	}
	if _, ok := payload["autoUpdate"]; !ok {
		t.Error("expected autoUpdate block in payload")
	}
	if _, ok := payload["envVars"]; !ok {
		t.Error("expected envVars in payload")
	}
	// webhook_id is computed when stack_webhook is set.
	if d.Get("webhook_id").(string) == "" {
		t.Error("expected computed webhook_id to be set")
	}
}

// TestEdgeStackCov2_Update_FileContent_HTTPError covers the >=300 branch of the
// file/content update path.
func TestEdgeStackCov2_Update_FileContent_HTTPError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/edge_stacks/42", RespondString(http.StatusBadRequest, "application/json", `{"message":"bad"}`))

	r := resourceEdgeStack()
	d := r.TestResourceData()
	d.SetId("42")
	_ = d.Set("name", "x")
	_ = d.Set("deployment_type", 0)
	_ = d.Set("edge_groups", []interface{}{1})
	_ = d.Set("stack_file_content", "version: \"3\"\n")

	if err := rcUpdate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
}

// TestEdgeStackCov2_Update_Repository_HTTPError covers the >=300 branch of the
// git update path.
func TestEdgeStackCov2_Update_Repository_HTTPError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/edge_stacks/55/git", RespondString(http.StatusBadRequest, "application/json", `{"message":"bad"}`))

	r := resourceEdgeStack()
	d := r.TestResourceData()
	d.SetId("55")
	_ = d.Set("name", "x")
	_ = d.Set("deployment_type", 0)
	_ = d.Set("edge_groups", []interface{}{1})
	_ = d.Set("repository_url", "https://github.com/example/repo.git")
	_ = d.Set("repository_reference_name", "refs/heads/main")

	if err := rcUpdate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
}

// TestEdgeStackCov2_Update_NoSourceError covers the final error branch of Update
// when no source is set.
func TestEdgeStackCov2_Update_NoSourceError(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceEdgeStack()
	d := r.TestResourceData()
	d.SetId("9")
	_ = d.Set("name", "x")
	_ = d.Set("deployment_type", 0)
	_ = d.Set("edge_groups", []interface{}{1})

	if err := rcUpdate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when no source provided for update, got nil")
	}
}

// TestEdgeStackCov2_Update_Repository_WithAuthAndWebhook exercises the git
// update path including auth block, relative_path and autoUpdate/webhook, then
// asserts webhook outputs computed and Read chained.
func TestEdgeStackCov2_Update_Repository_WithAuthAndWebhook(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/edge_stacks/60/git", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 60}))
	mock.On("GET", "/edge_stacks/60", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   60,
		"Name": "git-upd",
		// Readback must echo the GitOps webhook, otherwise Read drops the
		// computed webhook_id set during Update.
		"AutoUpdate": map[string]interface{}{
			"Interval": "5m",
			"Webhook":  "wh-uuid-60",
		},
	}))

	r := resourceEdgeStack()
	d := r.TestResourceData()
	d.SetId("60")
	_ = d.Set("name", "git-upd")
	_ = d.Set("deployment_type", 0)
	_ = d.Set("edge_groups", []interface{}{9})
	_ = d.Set("repository_url", "https://github.com/example/repo.git")
	_ = d.Set("repository_reference_name", "refs/heads/main")
	_ = d.Set("git_repository_authentication", true)
	_ = d.Set("repository_username", "bot")
	_ = d.Set("repository_password", "secret")
	_ = d.Set("relative_path", "stacks/app")
	_ = d.Set("stack_webhook", true)

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/edge_stacks/60/git")
	if put == nil {
		t.Fatal("expected PUT /edge_stacks/60/git")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if _, ok := payload["authentication"]; !ok {
		t.Error("expected authentication block in payload")
	}
	if payload["supportRelativePath"] != true {
		t.Errorf("expected supportRelativePath true, got %v", payload["supportRelativePath"])
	}
	if _, ok := payload["autoUpdate"]; !ok {
		t.Error("expected autoUpdate block in payload")
	}
	if d.Get("webhook_id").(string) == "" {
		t.Error("expected computed webhook_id after update")
	}
}

// TestEdgeStackCov2_Read_HTTPError covers the non-404 error branch of Read.
func TestEdgeStackCov2_Read_HTTPError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_stacks/9", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceEdgeStack()
	d := r.TestResourceData()
	d.SetId("9")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestEdgeStackCov2_Read_GitConfigNoAuth covers the Read GitConfig branch with
// no authentication block (git_repository_authentication set false).
func TestEdgeStackCov2_Read_GitConfigNoAuth(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_stacks/70", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   70,
		"Name": "git-read",
		"GitConfig": map[string]interface{}{
			"URL":            "https://github.com/example/repo.git",
			"ReferenceName":  "refs/heads/main",
			"ConfigFilePath": "docker-compose.yml",
		},
		"SupportRelativePath": true,
		"FilesystemPath":      "stacks/app",
	}))

	r := resourceEdgeStack()
	d := r.TestResourceData()
	d.SetId("70")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("repository_url"); got != "https://github.com/example/repo.git" {
		t.Errorf("repository_url: got %v", got)
	}
	if got := d.Get("git_repository_authentication"); got != false {
		t.Errorf("git_repository_authentication: expected false, got %v", got)
	}
	if got := d.Get("relative_path"); got != "stacks/app" {
		t.Errorf("relative_path: got %v", got)
	}
}

// TestEdgeStackCov2_Delete_HTTPError covers the error branch of Delete (non-204,
// non-404).
func TestEdgeStackCov2_Delete_HTTPError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("DELETE", "/edge_stacks/9", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceEdgeStack()
	d := r.TestResourceData()
	d.SetId("9")

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500 delete, got nil")
	}
}
