package internal

import (
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// resource_edge_stack.go uses raw http.NewRequestWithContext + client.HTTPClient.Do
// for every CRUD operation (not the generated SDK). The mock harness drives this
// transparently because client.Endpoint == mock.URL, so the dispatcher sees the
// bare paths (e.g. "/edge_stacks/create/file") with no "/api" prefix.
//
// Three mutually-exclusive create variants exist, selected by which optional
// field is set (checked in this order inside resourceEdgeStackCreate):
//
//  1. stack_file_content  -> POST /edge_stacks/create/string   (JSON, camelCase)
//  2. stack_file_path      -> POST /edge_stacks/create/file     (multipart, PascalCase form fields + "file" part)
//  3. repository_url        -> POST /edge_stacks/create/repository (JSON, camelCase)
//
// Every Create path FIRST calls findExistingEdgeStackByName, which does a
// GET /edge_stacks (list). If a stack with the same Name already exists, Create
// short-circuits into Update. So every happy-path Create test must register the
// list mock returning either an empty list or a list without the target name.
//
// On success the create response {"Id":N} sets the ID and chains into Read,
// which does GET /edge_stacks/{id}. So Create tests must also register that Read.

// parseMultipart decodes a recorded multipart request body into a map of form
// fields plus the captured "file" part (filename + content). It reads the
// boundary from the Content-Type header recorded by the mock harness.
func parseMultipart(t *testing.T, req *RecordedRequest) (fields map[string]string, fileName, fileContent string) {
	t.Helper()
	ct := req.Headers.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(ct)
	if err != nil {
		t.Fatalf("failed to parse Content-Type %q: %v", ct, err)
	}
	if !strings.HasPrefix(mediaType, "multipart/") {
		t.Fatalf("expected multipart Content-Type, got %q", mediaType)
	}
	boundary, ok := params["boundary"]
	if !ok {
		t.Fatalf("multipart Content-Type missing boundary: %q", ct)
	}

	fields = map[string]string{}
	mr := multipart.NewReader(strings.NewReader(string(req.Body)), boundary)
	for {
		part, err := mr.NextPart()
		if err != nil {
			break // io.EOF when done
		}
		buf := new(strings.Builder)
		_, _ = copyAll(buf, part)
		if part.FileName() != "" {
			fileName = part.FileName()
			fileContent = buf.String()
		} else {
			fields[part.FormName()] = buf.String()
		}
		_ = part.Close()
	}
	return fields, fileName, fileContent
}

// copyAll is a tiny io.Copy wrapper kept local so the test file does not need
// to import io directly for a single use.
func copyAll(dst *strings.Builder, src interface{ Read([]byte) (int, error) }) (int, error) {
	total := 0
	b := make([]byte, 4096)
	for {
		n, err := src.Read(b)
		if n > 0 {
			dst.Write(b[:n])
			total += n
		}
		if err != nil {
			return total, nil
		}
	}
}

// TestEdgeStackCreateMultipart_HappyPath covers the stack_file_path (file
// upload) create variant. It verifies:
//   - the request goes to POST /edge_stacks/create/file as multipart/form-data
//   - the PascalCase form fields carry the expected values
//   - the "file" part carries the on-disk stack file content
//   - the response Id (5) is stored and Create chains into Read
func TestEdgeStackCreateMultipart_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	// findExistingEdgeStackByName: list is empty so Create proceeds (no short-circuit).
	mock.On("GET", "/edge_stacks", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/edge_stacks/create/file", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   5,
		"Name": "edge-web",
	}))
	// Create chains into Read.
	mock.On("GET", "/edge_stacks/5", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   5,
		"Name": "edge-web",
	}))

	// Write a temp stack file.
	dir := t.TempDir()
	stackPath := filepath.Join(dir, "docker-compose.yml")
	stackContent := "version: \"3\"\nservices:\n  web:\n    image: nginx\n"
	if err := os.WriteFile(stackPath, []byte(stackContent), 0o600); err != nil {
		t.Fatalf("failed to write temp stack file: %v", err)
	}

	r := resourceEdgeStack()
	d := r.TestResourceData()
	_ = d.Set("name", "edge-web")
	_ = d.Set("deployment_type", 0)
	_ = d.Set("edge_groups", []interface{}{1, 2})
	_ = d.Set("registries", []interface{}{7})
	_ = d.Set("pre_pull_image", true)
	_ = d.Set("retry_deploy", true)
	_ = d.Set("stack_file_path", stackPath)

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "5" {
		t.Errorf("expected ID %q, got %q", "5", d.Id())
	}

	post := mock.FindRequest("POST", "/edge_stacks/create/file")
	if post == nil {
		t.Fatal("expected POST /edge_stacks/create/file to be sent")
	}

	fields, fileName, fileContent := parseMultipart(t, post)
	if fields["Name"] != "edge-web" {
		t.Errorf("form field Name: expected %q, got %q", "edge-web", fields["Name"])
	}
	if fields["DeploymentType"] != "0" {
		t.Errorf("form field DeploymentType: expected %q, got %q", "0", fields["DeploymentType"])
	}
	// EdgeGroups is JSON-encoded ([1,2]).
	if fields["EdgeGroups"] != "[1,2]" {
		t.Errorf("form field EdgeGroups: expected %q, got %q", "[1,2]", fields["EdgeGroups"])
	}
	if fields["Registries"] != "[7]" {
		t.Errorf("form field Registries: expected %q, got %q", "[7]", fields["Registries"])
	}
	if fields["PrePullImage"] != "true" {
		t.Errorf("form field PrePullImage: expected %q, got %q", "true", fields["PrePullImage"])
	}
	if fields["RetryDeploy"] != "true" {
		t.Errorf("form field RetryDeploy: expected %q, got %q", "true", fields["RetryDeploy"])
	}
	if fields["UseManifestNamespaces"] != "false" {
		t.Errorf("form field UseManifestNamespaces: expected %q, got %q", "false", fields["UseManifestNamespaces"])
	}
	if fileName != "docker-compose.yml" {
		t.Errorf("expected file part filename %q, got %q", "docker-compose.yml", fileName)
	}
	if fileContent != stackContent {
		t.Errorf("file part content mismatch:\n expected %q\n got      %q", stackContent, fileContent)
	}

	if mock.FindRequest("GET", "/edge_stacks/5") == nil {
		t.Error("expected Create to chain into Read at GET /edge_stacks/5")
	}
}

// TestEdgeStackCreateString_HappyPath covers the stack_file_content (inline
// string) create variant: POST /edge_stacks/create/string with a JSON payload
// using camelCase field names. Verifies payload fields, ID, and Read chain.
func TestEdgeStackCreateString_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/edge_stacks", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/edge_stacks/create/string", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   11,
		"Name": "edge-inline",
	}))
	mock.On("GET", "/edge_stacks/11", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   11,
		"Name": "edge-inline",
	}))

	r := resourceEdgeStack()
	d := r.TestResourceData()
	_ = d.Set("name", "edge-inline")
	_ = d.Set("deployment_type", 1)
	_ = d.Set("edge_groups", []interface{}{3})
	_ = d.Set("registries", []interface{}{})
	_ = d.Set("use_manifest_namespaces", true)
	_ = d.Set("stack_file_content", "version: \"3\"\nservices: {}\n")
	_ = d.Set("environment", map[string]interface{}{"FOO": "bar"})

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "11" {
		t.Errorf("expected ID %q, got %q", "11", d.Id())
	}

	post := mock.FindRequest("POST", "/edge_stacks/create/string")
	if post == nil {
		t.Fatal("expected POST /edge_stacks/create/string to be sent")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode create/string body: %v", err)
	}
	if got := payload["name"]; got != "edge-inline" {
		t.Errorf("payload.name: expected %q, got %v", "edge-inline", got)
	}
	// JSON numbers decode as float64.
	if got := payload["deploymentType"]; got != float64(1) {
		t.Errorf("payload.deploymentType: expected 1, got %v", got)
	}
	if got := payload["stackFileContent"]; got != "version: \"3\"\nservices: {}\n" {
		t.Errorf("payload.stackFileContent mismatch, got %v", got)
	}
	if got := payload["useManifestNamespaces"]; got != true {
		t.Errorf("payload.useManifestNamespaces: expected true, got %v", got)
	}
	edgeGroups, ok := payload["edgeGroups"].([]interface{})
	if !ok || len(edgeGroups) != 1 || edgeGroups[0] != float64(3) {
		t.Errorf("payload.edgeGroups: expected [3], got %v", payload["edgeGroups"])
	}
	// envVars carries the environment map as a list of {name,value}.
	envVars, ok := payload["envVars"].([]interface{})
	if !ok || len(envVars) != 1 {
		t.Fatalf("payload.envVars: expected one entry, got %v", payload["envVars"])
	}
	env0 := envVars[0].(map[string]interface{})
	if env0["name"] != "FOO" || env0["value"] != "bar" {
		t.Errorf("payload.envVars[0]: expected {FOO bar}, got %v", env0)
	}

	if mock.FindRequest("GET", "/edge_stacks/11") == nil {
		t.Error("expected Create to chain into Read at GET /edge_stacks/11")
	}
}

// TestEdgeStackCreateRepository_HappyPath covers the repository_url (git) create
// variant: POST /edge_stacks/create/repository with a JSON payload. The git auth
// chain is exercised at a basic level (no relative-path / webhook complexity) to
// keep the test focused on the request shape and the Create->Read chain.
func TestEdgeStackCreateRepository_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/edge_stacks", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/edge_stacks/create/repository", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   21,
		"Name": "edge-git",
	}))
	mock.On("GET", "/edge_stacks/21", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   21,
		"Name": "edge-git",
	}))

	r := resourceEdgeStack()
	d := r.TestResourceData()
	_ = d.Set("name", "edge-git")
	_ = d.Set("deployment_type", 0)
	_ = d.Set("edge_groups", []interface{}{4})
	_ = d.Set("repository_url", "https://github.com/example/repo.git")
	_ = d.Set("repository_reference_name", "refs/heads/main")
	_ = d.Set("file_path_in_repository", "docker-compose.yml")

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "21" {
		t.Errorf("expected ID %q, got %q", "21", d.Id())
	}

	post := mock.FindRequest("POST", "/edge_stacks/create/repository")
	if post == nil {
		t.Fatal("expected POST /edge_stacks/create/repository to be sent")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode create/repository body: %v", err)
	}
	if got := payload["name"]; got != "edge-git" {
		t.Errorf("payload.name: expected %q, got %v", "edge-git", got)
	}
	if got := payload["repositoryURL"]; got != "https://github.com/example/repo.git" {
		t.Errorf("payload.repositoryURL: expected the git URL, got %v", got)
	}
	if got := payload["repositoryReferenceName"]; got != "refs/heads/main" {
		t.Errorf("payload.repositoryReferenceName: expected %q, got %v", "refs/heads/main", got)
	}
	if got := payload["filePathInRepository"]; got != "docker-compose.yml" {
		t.Errorf("payload.filePathInRepository: expected %q, got %v", "docker-compose.yml", got)
	}

	if mock.FindRequest("GET", "/edge_stacks/21") == nil {
		t.Error("expected Create to chain into Read at GET /edge_stacks/21")
	}
}

// TestEdgeStackCreate_ExistingName_DelegatesToUpdate verifies the
// findExistingEdgeStackByName short-circuit: when a stack with the requested
// Name already exists in the GET /edge_stacks list, Create sets the ID to the
// existing record and delegates to Update instead of POSTing a new stack.
func TestEdgeStackCreate_ExistingName_DelegatesToUpdate(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/edge_stacks", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 30, "Name": "edge-web"},
	}))
	// Update with stack_file_content goes to PUT /edge_stacks/{id}.
	mock.On("PUT", "/edge_stacks/30", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   30,
		"Name": "edge-web",
	}))
	mock.On("GET", "/edge_stacks/30", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   30,
		"Name": "edge-web",
	}))

	r := resourceEdgeStack()
	d := r.TestResourceData()
	_ = d.Set("name", "edge-web")
	_ = d.Set("deployment_type", 0)
	_ = d.Set("edge_groups", []interface{}{1})
	_ = d.Set("stack_file_content", "version: \"3\"\n")

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create (existing-name path) failed: %v", err)
	}

	if d.Id() != "30" {
		t.Errorf("expected ID %q (reused from existing stack), got %q", "30", d.Id())
	}
	if mock.FindRequest("POST", "/edge_stacks/create/string") != nil {
		t.Error("expected NO POST create when name already exists, but one was sent")
	}
	if mock.FindRequest("PUT", "/edge_stacks/30") == nil {
		t.Error("expected PUT /edge_stacks/30 (Update delegation) to be sent")
	}
}

// TestEdgeStackRead_HappyPath verifies a successful GET /edge_stacks/{id}
// hydrates the relevant state fields (name, environment, autoUpdate-derived
// fields, always_clone).
func TestEdgeStackRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/edge_stacks/8", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   8,
		"Name": "edge-read",
		"envVars": []map[string]interface{}{
			{"name": "KEY", "value": "VAL"},
		},
		"AutoUpdate": map[string]interface{}{
			"Interval":       "5m",
			"Webhook":        "wh-123",
			"ForcePullImage": true,
		},
		"AlwaysCloneGitRepoForRelativePath": true,
	}))

	r := resourceEdgeStack()
	d := r.TestResourceData()
	d.SetId("8")

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "8" {
		t.Errorf("expected ID to remain %q, got %q", "8", d.Id())
	}
	if got := d.Get("name"); got != "edge-read" {
		t.Errorf("name: expected %q, got %v", "edge-read", got)
	}
	env := d.Get("environment").(map[string]interface{})
	if env["KEY"] != "VAL" {
		t.Errorf("environment[KEY]: expected %q, got %v", "VAL", env["KEY"])
	}
	if got := d.Get("pull_image"); got != true {
		t.Errorf("pull_image: expected true (from AutoUpdate.ForcePullImage), got %v", got)
	}
	if got := d.Get("update_interval"); got != "5m" {
		t.Errorf("update_interval: expected %q, got %v", "5m", got)
	}
	if got := d.Get("always_clone"); got != true {
		t.Errorf("always_clone: expected true, got %v", got)
	}
}

// TestEdgeStackRead_404_ClearsID confirms the 404 branch in Read removes the
// resource from state (standard Terraform drift-detection pattern).
func TestEdgeStackRead_404_ClearsID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/edge_stacks/77", RespondString(
		http.StatusNotFound, "application/json",
		`{"message":"edge stack not found"}`,
	))

	r := resourceEdgeStack()
	d := r.TestResourceData()
	d.SetId("77")

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read should swallow 404 and clear ID, got error: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared after 404, got %q", d.Id())
	}
}

// TestEdgeStackUpdate_FileContent_HappyPath covers the file/content update path:
// when stack_file_content is set, Update sends PUT /edge_stacks/{id} with a JSON
// body and then chains into Read.
func TestEdgeStackUpdate_FileContent_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/edge_stacks/42", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   42,
		"Name": "edge-upd",
	}))
	mock.On("GET", "/edge_stacks/42", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   42,
		"Name": "edge-upd",
	}))

	r := resourceEdgeStack()
	d := r.TestResourceData()
	d.SetId("42")
	_ = d.Set("name", "edge-upd")
	_ = d.Set("deployment_type", 0)
	_ = d.Set("edge_groups", []interface{}{1, 2})
	_ = d.Set("registries", []interface{}{})
	_ = d.Set("stack_file_content", "version: \"3\"\nservices:\n  app: {}\n")

	if err := r.Update(d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/edge_stacks/42")
	if put == nil {
		t.Fatal("expected PUT /edge_stacks/42 to be sent")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode PUT body: %v", err)
	}
	if got := payload["name"]; got != "edge-upd" {
		t.Errorf("payload.name: expected %q, got %v", "edge-upd", got)
	}
	if got := payload["updateVersion"]; got != true {
		t.Errorf("payload.updateVersion: expected true, got %v", got)
	}
	if got := payload["stackFileContent"]; got != "version: \"3\"\nservices:\n  app: {}\n" {
		t.Errorf("payload.stackFileContent mismatch, got %v", got)
	}
	if got := payload["deploymentType"]; got != float64(0) {
		t.Errorf("payload.deploymentType: expected 0, got %v", got)
	}

	if mock.FindRequest("GET", "/edge_stacks/42") == nil {
		t.Error("expected Update to chain into Read at GET /edge_stacks/42")
	}
}

// TestEdgeStackUpdate_Repository_HappyPath covers the git update path: when only
// repository_url is set (no stack_file_content / stack_file_path), Update sends
// PUT /edge_stacks/{id}/git with a JSON body using the git-specific field names
// (groupIds, refName), then chains into Read.
func TestEdgeStackUpdate_Repository_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/edge_stacks/55/git", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   55,
		"Name": "edge-git-upd",
	}))
	mock.On("GET", "/edge_stacks/55", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   55,
		"Name": "edge-git-upd",
	}))

	r := resourceEdgeStack()
	d := r.TestResourceData()
	d.SetId("55")
	_ = d.Set("name", "edge-git-upd")
	_ = d.Set("deployment_type", 0)
	_ = d.Set("edge_groups", []interface{}{9})
	_ = d.Set("registries", []interface{}{})
	_ = d.Set("repository_url", "https://github.com/example/repo.git")
	_ = d.Set("repository_reference_name", "refs/heads/main")

	if err := r.Update(d, mock.Client()); err != nil {
		t.Fatalf("Update (repository) failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/edge_stacks/55/git")
	if put == nil {
		t.Fatal("expected PUT /edge_stacks/55/git to be sent")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode PUT /git body: %v", err)
	}
	if got := payload["refName"]; got != "refs/heads/main" {
		t.Errorf("payload.refName: expected %q, got %v", "refs/heads/main", got)
	}
	groupIds, ok := payload["groupIds"].([]interface{})
	if !ok || len(groupIds) != 1 || groupIds[0] != float64(9) {
		t.Errorf("payload.groupIds: expected [9], got %v", payload["groupIds"])
	}
	if got := payload["updateVersion"]; got != true {
		t.Errorf("payload.updateVersion: expected true, got %v", got)
	}

	// The file/content PUT must NOT have been sent.
	if mock.FindRequest("PUT", "/edge_stacks/55") != nil {
		t.Error("expected NO plain PUT /edge_stacks/55 for repository update; git path should be used")
	}
	if mock.FindRequest("GET", "/edge_stacks/55") == nil {
		t.Error("expected Update to chain into Read at GET /edge_stacks/55")
	}
}

// TestEdgeStackDelete_HappyPath verifies DELETE /edge_stacks/{id} is sent and a
// 204 response is treated as success.
func TestEdgeStackDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/edge_stacks/5", RespondString(http.StatusNoContent, "", ""))

	r := resourceEdgeStack()
	d := r.TestResourceData()
	d.SetId("5")

	if err := r.Delete(d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/edge_stacks/5") == nil {
		t.Error("expected DELETE /edge_stacks/5 to be sent")
	}
}

// TestEdgeStackDelete_404_NoError verifies a 404 on delete is swallowed (the
// stack was already gone).
func TestEdgeStackDelete_404_NoError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/edge_stacks/99", RespondString(
		http.StatusNotFound, "application/json",
		`{"message":"edge stack not found"}`,
	))

	r := resourceEdgeStack()
	d := r.TestResourceData()
	d.SetId("99")

	if err := r.Delete(d, mock.Client()); err != nil {
		t.Fatalf("Delete should swallow 404, got error: %v", err)
	}
}

// TestEdgeStackCreate_HTTPError ensures a non-2xx response from the file-create
// endpoint surfaces as an error and leaves the resource ID empty.
func TestEdgeStackCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/edge_stacks", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/edge_stacks/create/file", RespondString(
		http.StatusBadRequest, "application/json",
		`{"message":"invalid edge stack payload"}`,
	))

	dir := t.TempDir()
	stackPath := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(stackPath, []byte("version: \"3\"\n"), 0o600); err != nil {
		t.Fatalf("failed to write temp stack file: %v", err)
	}

	r := resourceEdgeStack()
	d := r.TestResourceData()
	_ = d.Set("name", "broken")
	_ = d.Set("deployment_type", 0)
	_ = d.Set("edge_groups", []interface{}{1})
	_ = d.Set("stack_file_path", stackPath)

	if err := r.Create(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}
