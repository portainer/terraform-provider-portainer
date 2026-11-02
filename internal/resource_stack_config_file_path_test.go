package internal

import (
	"net/http"
	"strings"
	"testing"
)

// file_path_in_repository is updatable, but Update never sent it, so a changed
// path was planned on every run while the stack kept deploying the old file.
// Portainer 2.42 and later apply it through POST /stacks/{id}/git; an older
// version ignores the field, which Update reports instead of looping.

func stackConfigFilePathMock(t *testing.T, returnedPath string) *MockServer {
	mock := NewMockServer(t)
	mock.On("POST", "/stacks/94/git", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":        94,
		"GitConfig": map[string]interface{}{"ConfigFilePath": returnedPath},
	}))
	mock.On("PUT", "/stacks/94/git/redeploy", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/stacks/94", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 94, "Name": "app", "Status": 1, "Type": 2, "EndpointId": 1,
		"GitConfig": map[string]interface{}{
			"URL":            "https://github.com/acme/app.git",
			"ReferenceName":  "refs/heads/main",
			"ConfigFilePath": returnedPath,
		},
	}))
	return mock
}

func TestStackUpdate_SendsConfigFilePath(t *testing.T) {
	mock := stackConfigFilePathMock(t, "deploy/compose.prod.yml")
	r := resourcePortainerStack()
	d := r.TestResourceData()
	d.SetId("94")
	_ = d.Set("method", "repository")
	_ = d.Set("name", "app")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("repository_url", "https://github.com/acme/app.git")
	_ = d.Set("file_path_in_repository", "deploy/compose.prod.yml")
	_ = d.Set("active", true)

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	var payload map[string]interface{}
	if err := mock.FindRequest("POST", "/stacks/94/git").DecodeJSON(&payload); err != nil {
		t.Fatalf("decode git payload: %v", err)
	}
	if payload["configFilePath"] != "deploy/compose.prod.yml" {
		t.Errorf("payload.configFilePath: expected the new path, got %v", payload["configFilePath"])
	}
	if got := d.Get("file_path_in_repository"); got != "deploy/compose.prod.yml" {
		t.Errorf("file_path_in_repository: expected the new path, got %v", got)
	}
}

func TestStackUpdate_ConfigFilePathIgnoredByOldPortainer(t *testing.T) {
	mock := stackConfigFilePathMock(t, "docker-compose.yml")
	r := resourcePortainerStack()
	d := r.TestResourceData()
	d.SetId("94")
	_ = d.Set("method", "repository")
	_ = d.Set("name", "app")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("repository_url", "https://github.com/acme/app.git")
	_ = d.Set("file_path_in_repository", "deploy/compose.prod.yml")
	_ = d.Set("active", true)

	err := rcUpdate(r, d, mock.Client())
	if err == nil || !strings.Contains(err.Error(), "cannot change file_path_in_repository in place") {
		t.Fatalf("expected an in-place change error, got %v", err)
	}
	if mock.FindRequest("PUT", "/stacks/94/git/redeploy") != nil {
		t.Error("expected no redeploy after Portainer ignored the new path")
	}
}
