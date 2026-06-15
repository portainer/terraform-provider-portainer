package internal

import (
	"net/http"
	"testing"
)

// =========================================================================
// Additional coverage for resource_custom_template.go targeting paths not in
// resource_custom_template_test.go: Read 404 -> clear ID, Create error
// propagation, Create with no source (validation error), the repository create
// path, the Update happy path (incl. git_fetch for git-based templates),
// Delete 404 swallow, and getVariablesSDK with variables present.
// =========================================================================

// TestCustomTemplateRead_404ClearsID verifies a NotFound on inspect clears the
// resource ID for drift detection.
func TestCustomTemplateRead_404ClearsID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/custom_templates/55", RespondString(
		http.StatusNotFound, "application/json", `{"message":"not found"}`,
	))

	r := resourceCustomTemplate()
	d := r.TestResourceData()
	d.SetId("55")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read should swallow 404 and clear ID, got: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared after 404, got %q", d.Id())
	}
}

// TestCustomTemplateCreate_FromString_HTTPError verifies an error from the
// create endpoint propagates.
func TestCustomTemplateCreate_FromString_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/custom_templates", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/custom_templates/create/string", RespondString(
		http.StatusBadRequest, "application/json", `{"message":"bad template"}`,
	))

	r := resourceCustomTemplate()
	d := r.TestResourceData()
	_ = d.Set("title", "boom")
	_ = d.Set("description", "d")
	_ = d.Set("note", "n")
	_ = d.Set("platform", 1)
	_ = d.Set("type", 1)
	_ = d.Set("file_content", "x")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
}

// TestCustomTemplateCreate_NoSource verifies the validation error when none of
// file_content, file_path, or repository_url is provided.
func TestCustomTemplateCreate_NoSource(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/custom_templates", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	r := resourceCustomTemplate()
	d := r.TestResourceData()
	_ = d.Set("title", "nosrc")
	_ = d.Set("description", "d")
	_ = d.Set("note", "n")
	_ = d.Set("platform", 1)
	_ = d.Set("type", 1)
	// no source set

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when no template source provided, got nil")
	}
}

// TestCustomTemplateCreate_FromRepository covers createTemplateFromRepository,
// including the repository_authentication=true branch.
func TestCustomTemplateCreate_FromRepository(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/custom_templates", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/custom_templates/create/repository", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":    88,
		"Title": "gittmpl",
	}))

	r := resourceCustomTemplate()
	d := r.TestResourceData()
	_ = d.Set("title", "gittmpl")
	_ = d.Set("description", "d")
	_ = d.Set("note", "n")
	_ = d.Set("platform", 1)
	_ = d.Set("type", 1)
	_ = d.Set("repository_url", "https://github.com/acme/tmpl.git")
	_ = d.Set("repository_authentication", true)
	_ = d.Set("repository_username", "robot")
	_ = d.Set("repository_password", "secret")
	_ = d.Set("compose_file_path", "docker-compose.yml")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "88" {
		t.Errorf("expected ID 88, got %q", d.Id())
	}
	post := mock.FindRequest("POST", "/custom_templates/create/repository")
	if post == nil {
		t.Fatal("expected POST /custom_templates/create/repository")
	}
}

// TestCustomTemplateUpdate_GitBased covers the Update path for a git-based
// template: PUT /custom_templates/{id} followed by PUT
// /custom_templates/{id}/git_fetch, then the chained Read.
func TestCustomTemplateUpdate_GitBased(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/custom_templates/90", RespondString(http.StatusOK, "application/json", `{}`))
	mock.On("PUT", "/custom_templates/90/git_fetch", RespondString(http.StatusOK, "application/json", `{}`))
	mock.On("GET", "/custom_templates/90", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":       90,
		"Title":    "gitupd",
		"Platform": 1,
		"Type":     1,
	}))

	r := resourceCustomTemplate()
	d := r.TestResourceData()
	d.SetId("90")
	_ = d.Set("title", "gitupd")
	_ = d.Set("description", "d")
	_ = d.Set("note", "n")
	_ = d.Set("platform", 1)
	_ = d.Set("type", 1)
	_ = d.Set("repository_url", "https://github.com/acme/tmpl.git")

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if mock.FindRequest("PUT", "/custom_templates/90") == nil {
		t.Error("expected PUT /custom_templates/90")
	}
	if mock.FindRequest("PUT", "/custom_templates/90/git_fetch") == nil {
		t.Error("expected PUT /custom_templates/90/git_fetch for git-based template")
	}
}

// TestCustomTemplateUpdate_NonGit covers the Update path for an inline template
// (no repository_url), which must NOT call git_fetch.
func TestCustomTemplateUpdate_NonGit(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/custom_templates/91", RespondString(http.StatusOK, "application/json", `{}`))
	mock.On("GET", "/custom_templates/91", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 91, "Title": "inlineupd", "Platform": 1, "Type": 1,
	}))

	r := resourceCustomTemplate()
	d := r.TestResourceData()
	d.SetId("91")
	_ = d.Set("title", "inlineupd")
	_ = d.Set("description", "d")
	_ = d.Set("note", "n")
	_ = d.Set("platform", 1)
	_ = d.Set("type", 1)
	_ = d.Set("file_content", "version: '3'")

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if mock.FindRequest("PUT", "/custom_templates/91/git_fetch") != nil {
		t.Error("did not expect git_fetch for an inline (non-git) template")
	}
}

// TestCustomTemplateDelete_404Swallowed verifies a NotFound on delete is
// treated as success.
func TestCustomTemplateDelete_404Swallowed(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/custom_templates/200", RespondString(
		http.StatusNotFound, "application/json", `{"message":"gone"}`,
	))

	r := resourceCustomTemplate()
	d := r.TestResourceData()
	d.SetId("200")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete should swallow 404, got: %v", err)
	}
}

// TestGetVariablesSDK covers getVariablesSDK with a populated variables list.
func TestGetVariablesSDK(t *testing.T) {
	r := resourceCustomTemplate()
	d := r.TestResourceData()
	_ = d.Set("variables", []interface{}{
		map[string]interface{}{
			"name":          "FOO",
			"label":         "Foo",
			"default_value": "bar",
			"description":   "the foo",
		},
	})

	vars := getVariablesSDK(d)
	if len(vars) != 1 {
		t.Fatalf("expected 1 variable, got %d", len(vars))
	}
	if vars[0].Name != "FOO" {
		t.Errorf("Name: expected FOO, got %q", vars[0].Name)
	}
	if vars[0].DefaultValue != "bar" {
		t.Errorf("DefaultValue: expected bar, got %q", vars[0].DefaultValue)
	}
}

// TestGetVariablesSDK_Empty returns nil when no variables are set.
func TestGetVariablesSDK_Empty(t *testing.T) {
	r := resourceCustomTemplate()
	d := r.TestResourceData()
	if got := getVariablesSDK(d); got != nil {
		t.Errorf("expected nil for empty variables, got %v", got)
	}
}
