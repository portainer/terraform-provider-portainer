package internal

import (
	"net/http"
	"testing"
)

// source_id (Portainer 2.43+) lets a custom template or an edge stack take its
// repository URL and credentials from a Portainer Source instead of inline
// fields, so neither needs repository_url.

func TestCustomTemplateCreate_FromSource(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/custom_templates", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/custom_templates/create/repository", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 91, "Title": "src"}))
	mock.On("GET", "/custom_templates/91", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 91, "Title": "src"}))

	r := resourceCustomTemplate()
	d := r.TestResourceData()
	_ = d.Set("title", "src")
	_ = d.Set("description", "d")
	_ = d.Set("note", "n")
	_ = d.Set("platform", 1)
	_ = d.Set("type", 2)
	_ = d.Set("source_id", 4)
	_ = d.Set("compose_file_path", "deploy/compose.yml")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "91" {
		t.Errorf("expected ID 91, got %q", d.Id())
	}
	var payload map[string]interface{}
	if err := mock.FindRequest("POST", "/custom_templates/create/repository").DecodeJSON(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload["SourceID"] != float64(4) {
		t.Errorf("payload.SourceID: expected 4, got %v", payload["SourceID"])
	}
	if _, ok := payload["repositoryURL"]; ok {
		t.Errorf("payload.repositoryURL: expected it left out for a Source, got %v", payload["repositoryURL"])
	}
	if payload["composeFilePathInRepository"] != "deploy/compose.yml" {
		t.Errorf("payload.composeFilePathInRepository: expected the configured path, got %v", payload["composeFilePathInRepository"])
	}
}

func TestCustomTemplateUpdate_FromSourceSendsSourceID(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/custom_templates/91", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 91}))
	mock.On("PUT", "/custom_templates/91/git_fetch", RespondJSON(http.StatusOK, map[string]interface{}{"FileContent": ""}))
	mock.On("GET", "/custom_templates/91", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 91, "Title": "src"}))

	r := resourceCustomTemplate()
	d := r.TestResourceData()
	d.SetId("91")
	_ = d.Set("title", "src")
	_ = d.Set("description", "changed")
	_ = d.Set("note", "n")
	_ = d.Set("platform", 1)
	_ = d.Set("type", 2)
	_ = d.Set("source_id", 4)

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	var payload map[string]interface{}
	if err := mock.FindRequest("PUT", "/custom_templates/91").DecodeJSON(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload["SourceID"] != float64(4) {
		t.Errorf("payload.SourceID: expected 4 on every update, got %v", payload["SourceID"])
	}
}

func TestEdgeStackCreate_FromSource(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_stacks", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/edge_stacks/create/repository", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 32, "Name": "src"}))
	mock.On("GET", "/edge_stacks/32", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 32, "Name": "src"}))

	r := resourceEdgeStack()
	d := r.TestResourceData()
	_ = d.Set("name", "src")
	_ = d.Set("edge_groups", []interface{}{4})
	_ = d.Set("source_id", 6)
	_ = d.Set("file_path_in_repository", "edge/compose.yml")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	var payload map[string]interface{}
	if err := mock.FindRequest("POST", "/edge_stacks/create/repository").DecodeJSON(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload["sourceID"] != float64(6) {
		t.Errorf("payload.sourceID: expected 6, got %v", payload["sourceID"])
	}
}
