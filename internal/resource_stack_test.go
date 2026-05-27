package internal

import (
	"net/http"
	"strings"
	"testing"
)

// ===========================================================================
// CRUD tests for resource_stack.go.
//
// resource_stack uses raw http.NewRequest against client.Endpoint (mock.URL)
// + client.HTTPClient.Do — NOT client.DoRequest — so handler paths have no
// "/api" prefix (the dispatcher matches on path only; query strings live in
// req.Query). The few access-control side-paths DO use client.DoRequest, but
// those only fire when "ownership" is set; TestResourceData leaves it "" so
// updateStackAccessControl / readStackAccessControl short-circuit and never
// hit the wire.
//
// Create flow worth noting:
//   - findExistingStackByName always runs first => GET /stacks must be mocked
//     and return an empty list to take the "create new" path.
//   - For non-repository methods, after the create helper sets the ID, Create
//     performs a finalize PUT /stacks/{id} (prune/webhook) — must be mocked.
//   - Create then chains into Read: GET /stacks/{id}, and for non-repository
//     methods also GET /stacks/{id}/file.
//
// Not covered here (documented, intentionally skipped):
//   - method "file": reads from the local filesystem via os.ReadFile; covered
//     indirectly because it funnels into the same createStack*String helpers
//     after the read, which the "string" tests already exercise.
//   - Helm / k8s url / write-only (wo) repository credential variants: lower
//     usage; the k8s-string and repository happy paths exercise the shared
//     request machinery. The wo credentials require GetRawConfigAt, which is
//     not populated by TestResourceData.
//   - updateStackAccessControl / readStackAccessControl wire calls (require
//     ownership set + a resource_controls lookup chain).
// ===========================================================================

// All create/update/read/delete paths in resource_stack.go send JSON bodies,
// so tests assert via req.DecodeJSON. (No multipart helper is needed here,
// unlike resource_environment which uses the multipart SDK transport.)

// mockEmptyStackList registers the standard findExistingStackByName response
// that forces the "create new stack" path (no name collision).
func mockEmptyStackList(mock *MockServer) {
	mock.On("GET", "/stacks", RespondJSON(http.StatusOK, []map[string]interface{}{}))
}

// --------------- expandStringList ---------------

func TestExpandStringList(t *testing.T) {
	tests := []struct {
		name     string
		input    []interface{}
		expected []string
	}{
		{"normal", []interface{}{"a", "b", "c"}, []string{"a", "b", "c"}},
		{"empty", []interface{}{}, []string{}},
		{"single", []interface{}{"hello"}, []string{"hello"}},
		{"with empty strings", []interface{}{"", "a", ""}, []string{"", "a", ""}},
		{"unicode strings", []interface{}{"hello", "welt", "swiat"}, []string{"hello", "welt", "swiat"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandStringList(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected length %d, got %d", len(tt.expected), len(result))
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("at index %d: expected %q, got %q", i, tt.expected[i], v)
				}
			}
		})
	}
}

// --------------- expandIntList ---------------

func TestExpandIntList(t *testing.T) {
	tests := []struct {
		name     string
		input    []interface{}
		expected []int
	}{
		{"normal", []interface{}{1, 2, 3}, []int{1, 2, 3}},
		{"empty", []interface{}{}, []int{}},
		{"single", []interface{}{99}, []int{99}},
		{"with zero", []interface{}{0, 1, 0}, []int{0, 1, 0}},
		{"negative", []interface{}{-1, -2, -3}, []int{-1, -2, -3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandIntList(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected length %d, got %d", len(tt.expected), len(result))
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("at index %d: expected %d, got %d", i, tt.expected[i], v)
				}
			}
		})
	}
}

// --------------- flattenEnvList ---------------

func TestFlattenEnvList(t *testing.T) {
	t.Run("normal env list", func(t *testing.T) {
		input := []interface{}{
			map[string]interface{}{"name": "FOO", "value": "bar"},
			map[string]interface{}{"name": "BAZ", "value": "qux"},
		}
		result := flattenEnvList(input)
		if len(result) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(result))
		}
		if result[0]["name"] != "FOO" || result[0]["value"] != "bar" {
			t.Errorf("entry 0: expected FOO=bar, got %v", result[0])
		}
		if result[1]["name"] != "BAZ" || result[1]["value"] != "qux" {
			t.Errorf("entry 1: expected BAZ=qux, got %v", result[1])
		}
	})

	t.Run("empty env list", func(t *testing.T) {
		input := []interface{}{}
		result := flattenEnvList(input)
		if len(result) != 0 {
			t.Errorf("expected nil or empty, got %v", result)
		}
	})

	t.Run("single env entry", func(t *testing.T) {
		input := []interface{}{
			map[string]interface{}{"name": "KEY", "value": "val"},
		}
		result := flattenEnvList(input)
		if len(result) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(result))
		}
		if result[0]["name"] != "KEY" {
			t.Errorf("expected name=KEY, got %q", result[0]["name"])
		}
		if result[0]["value"] != "val" {
			t.Errorf("expected value=val, got %q", result[0]["value"])
		}
	})

	t.Run("env with empty values", func(t *testing.T) {
		input := []interface{}{
			map[string]interface{}{"name": "", "value": ""},
		}
		result := flattenEnvList(input)
		if len(result) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(result))
		}
		if result[0]["name"] != "" {
			t.Errorf("expected empty name, got %q", result[0]["name"])
		}
		if result[0]["value"] != "" {
			t.Errorf("expected empty value, got %q", result[0]["value"])
		}
	})
}

// =============================== CREATE ===============================

// TestStackCreate_StandaloneString_HappyPath covers the most common path:
// deployment_type=standalone, method=string with inline stack_file_content.
// Verifies the create POST endpoint, the finalize PUT, the Create→Read chain,
// the create payload fields, and that the ID is set from the create response.
func TestStackCreate_StandaloneString_HappyPath(t *testing.T) {
	mock := NewMockServer(t)
	mockEmptyStackList(mock)

	mock.On("POST", "/stacks/create/standalone/string", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   5,
		"Name": "web",
	}))
	// Finalize PUT (prune/webhook) that non-repository Create always sends.
	mock.On("PUT", "/stacks/5", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   5,
		"Name": "web",
	}))
	// Read chain.
	mock.On("GET", "/stacks/5", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":         5,
		"Name":       "web",
		"Status":     1,
		"Type":       2,
		"EndpointId": 1,
	}))
	mock.On("GET", "/stacks/5/file", RespondJSON(http.StatusOK, map[string]interface{}{
		"StackFileContent": "version: '3'",
	}))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("deployment_type", "standalone")
	_ = d.Set("method", "string")
	_ = d.Set("name", "web")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("stack_file_content", "version: '3'")

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "5" {
		t.Errorf("expected ID %q, got %q", "5", d.Id())
	}

	post := mock.FindRequest("POST", "/stacks/create/standalone/string")
	if post == nil {
		t.Fatal("expected POST /stacks/create/standalone/string to be sent")
	}
	// endpointId is passed as a query param, not in the path.
	if !strings.Contains(post.Query, "endpointId=1") {
		t.Errorf("expected create query to carry endpointId=1, got %q", post.Query)
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode create POST body: %v", err)
	}
	if got := payload["name"]; got != "web" {
		t.Errorf("payload.name: expected %q, got %v", "web", got)
	}
	if got := payload["stackFileContent"]; got != "version: '3'" {
		t.Errorf("payload.stackFileContent: expected the compose content, got %v", got)
	}
	if got := payload["fromAppTemplate"]; got != false {
		t.Errorf("payload.fromAppTemplate: expected false, got %v", got)
	}

	if mock.FindRequest("PUT", "/stacks/5") == nil {
		t.Error("expected finalize PUT /stacks/5 to be sent")
	}
	if mock.FindRequest("GET", "/stacks/5") == nil {
		t.Error("expected Create to chain into Read at GET /stacks/5")
	}
	if mock.FindRequest("GET", "/stacks/5/file") == nil {
		t.Error("expected Read to fetch stack file at GET /stacks/5/file")
	}

	// Read should populate active (Status 1 => active) and file content.
	if got := d.Get("active"); got != true {
		t.Errorf("active: expected true (Status=1), got %v", got)
	}
	if got := d.Get("stack_file_content"); got != "version: '3'" {
		t.Errorf("stack_file_content: expected populated from file endpoint, got %v", got)
	}
}

// TestStackCreate_SwarmString_HappyPath covers deployment_type=swarm,
// method=string. swarm_id is provided so fetchSwarmID is NOT triggered.
// Verifies the swarm create endpoint and that swarmID is carried in the
// payload.
func TestStackCreate_SwarmString_HappyPath(t *testing.T) {
	mock := NewMockServer(t)
	mockEmptyStackList(mock)

	mock.On("POST", "/stacks/create/swarm/string", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   8,
		"Name": "svc",
	}))
	mock.On("PUT", "/stacks/8", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 8, "Name": "svc"}))
	mock.On("GET", "/stacks/8", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":         8,
		"Name":       "svc",
		"Status":     1,
		"Type":       1,
		"SwarmId":    "swarm-abc",
		"EndpointId": 1,
	}))
	mock.On("GET", "/stacks/8/file", RespondJSON(http.StatusOK, map[string]interface{}{
		"StackFileContent": "version: '3'",
	}))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("deployment_type", "swarm")
	_ = d.Set("method", "string")
	_ = d.Set("name", "svc")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("swarm_id", "swarm-abc") // pre-set so fetchSwarmID is skipped
	_ = d.Set("stack_file_content", "version: '3'")

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "8" {
		t.Errorf("expected ID %q, got %q", "8", d.Id())
	}

	post := mock.FindRequest("POST", "/stacks/create/swarm/string")
	if post == nil {
		t.Fatal("expected POST /stacks/create/swarm/string to be sent")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode create POST body: %v", err)
	}
	if got := payload["swarmID"]; got != "swarm-abc" {
		t.Errorf("payload.swarmID: expected %q, got %v", "swarm-abc", got)
	}
	if got := payload["name"]; got != "svc" {
		t.Errorf("payload.name: expected %q, got %v", "svc", got)
	}

	// fetchSwarmID must NOT have been called because swarm_id was set.
	if mock.FindRequest("GET", "/endpoints/1/docker/swarm") != nil {
		t.Error("did not expect fetchSwarmID (GET /endpoints/1/docker/swarm) when swarm_id is pre-set")
	}
}

// TestStackCreate_StandaloneRepository_HappyPath covers the git/repository
// path. Repository creates do NOT send the finalize PUT and Read does NOT
// fetch the stack file. Verifies the repository POST endpoint and the
// repository fields in the payload.
func TestStackCreate_StandaloneRepository_HappyPath(t *testing.T) {
	mock := NewMockServer(t)
	mockEmptyStackList(mock)

	mock.On("POST", "/stacks/create/standalone/repository", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   11,
		"Name": "gitstack",
	}))
	mock.On("GET", "/stacks/11", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":         11,
		"Name":       "gitstack",
		"Status":     1,
		"Type":       2,
		"EndpointId": 1,
		"gitConfig": map[string]interface{}{
			"URL":            "https://github.com/acme/app.git",
			"ReferenceName":  "refs/heads/main",
			"ConfigFilePath": "docker-compose.yml",
		},
	}))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("deployment_type", "standalone")
	_ = d.Set("method", "repository")
	_ = d.Set("name", "gitstack")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("repository_url", "https://github.com/acme/app.git")
	_ = d.Set("repository_reference_name", "refs/heads/main")
	_ = d.Set("file_path_in_repository", "docker-compose.yml")

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "11" {
		t.Errorf("expected ID %q, got %q", "11", d.Id())
	}

	post := mock.FindRequest("POST", "/stacks/create/standalone/repository")
	if post == nil {
		t.Fatal("expected POST /stacks/create/standalone/repository to be sent")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode create POST body: %v", err)
	}
	if got := payload["repositoryURL"]; got != "https://github.com/acme/app.git" {
		t.Errorf("payload.repositoryURL: expected the git URL, got %v", got)
	}
	if got := payload["composeFile"]; got != "docker-compose.yml" {
		t.Errorf("payload.composeFile: expected docker-compose.yml, got %v", got)
	}
	if got := payload["repositoryReferenceName"]; got != "refs/heads/main" {
		t.Errorf("payload.repositoryReferenceName: expected refs/heads/main, got %v", got)
	}

	// Repository Create must NOT send the finalize PUT nor fetch the file.
	if mock.FindRequest("PUT", "/stacks/11") != nil {
		t.Error("did not expect finalize PUT /stacks/11 for repository method")
	}
	if mock.FindRequest("GET", "/stacks/11/file") != nil {
		t.Error("did not expect GET /stacks/11/file for repository method")
	}
	if mock.FindRequest("GET", "/stacks/11") == nil {
		t.Error("expected Create to chain into Read at GET /stacks/11")
	}
	if got := d.Get("repository_url"); got != "https://github.com/acme/app.git" {
		t.Errorf("repository_url: expected populated from gitConfig, got %v", got)
	}
}

// TestStackCreate_RepositoryDefaultsComposeFile verifies that when
// file_path_in_repository is empty, the repository create payload defaults the
// composeFile to docker-compose.yml.
func TestStackCreate_RepositoryDefaultsComposeFile(t *testing.T) {
	mock := NewMockServer(t)
	mockEmptyStackList(mock)

	mock.On("POST", "/stacks/create/standalone/repository", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 12, "Name": "gitdef",
	}))
	mock.On("GET", "/stacks/12", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 12, "Name": "gitdef", "Status": 1, "Type": 2, "EndpointId": 1,
	}))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("deployment_type", "standalone")
	_ = d.Set("method", "repository")
	_ = d.Set("name", "gitdef")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("repository_url", "https://github.com/acme/app.git")
	// file_path_in_repository intentionally left unset.

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	post := mock.FindRequest("POST", "/stacks/create/standalone/repository")
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode create POST body: %v", err)
	}
	if got := payload["composeFile"]; got != "docker-compose.yml" {
		t.Errorf("payload.composeFile: expected default docker-compose.yml, got %v", got)
	}
}

// TestStackCreate_KubernetesString_HappyPath covers deployment_type=kubernetes,
// method=string. The k8s string payload uses stackName/namespace fields.
func TestStackCreate_KubernetesString_HappyPath(t *testing.T) {
	mock := NewMockServer(t)
	mockEmptyStackList(mock)

	mock.On("POST", "/stacks/create/kubernetes/string", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 20, "Name": "k8sapp",
	}))
	mock.On("PUT", "/stacks/20", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 20, "Name": "k8sapp"}))
	mock.On("GET", "/stacks/20", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 20, "Name": "k8sapp", "Status": 1, "Type": 3, "EndpointId": 2, "namespace": "default",
	}))
	mock.On("GET", "/stacks/20/file", RespondJSON(http.StatusOK, map[string]interface{}{
		"StackFileContent": "apiVersion: v1",
	}))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("deployment_type", "kubernetes")
	_ = d.Set("method", "string")
	_ = d.Set("name", "k8sapp")
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("namespace", "default")
	_ = d.Set("stack_file_content", "apiVersion: v1")

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "20" {
		t.Errorf("expected ID %q, got %q", "20", d.Id())
	}
	post := mock.FindRequest("POST", "/stacks/create/kubernetes/string")
	if post == nil {
		t.Fatal("expected POST /stacks/create/kubernetes/string to be sent")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode create POST body: %v", err)
	}
	if got := payload["stackName"]; got != "k8sapp" {
		t.Errorf("payload.stackName: expected %q, got %v", "k8sapp", got)
	}
	if got := payload["namespace"]; got != "default" {
		t.Errorf("payload.namespace: expected %q, got %v", "default", got)
	}
}

// TestStackCreate_HTTPError ensures a non-200 from the create endpoint
// propagates as an error and leaves the resource ID empty.
func TestStackCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)
	mockEmptyStackList(mock)

	mock.On("POST", "/stacks/create/standalone/string", RespondString(
		http.StatusBadRequest, "application/json",
		`{"message":"invalid compose file"}`,
	))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("deployment_type", "standalone")
	_ = d.Set("method", "string")
	_ = d.Set("name", "broken")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("stack_file_content", "not-yaml")

	if err := r.Create(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}

// TestStackCreate_ExistingStackGuard covers the findExistingStackByName
// short-circuit: when a stack with the requested name+endpoint already exists,
// Create reuses the existing ID and delegates to Update instead of POSTing a
// new create. The non-repository Update path issues a PUT /stacks/{id}.
func TestStackCreate_ExistingStackGuard(t *testing.T) {
	mock := NewMockServer(t)

	// findExistingStackByName returns a matching stack.
	mock.On("GET", "/stacks", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 30, "Name": "web", "EndpointId": 1},
	}))
	// Update (non-repository) PUTs the stack content.
	mock.On("PUT", "/stacks/30", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 30, "Name": "web"}))
	// Update chains into Read.
	mock.On("GET", "/stacks/30", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 30, "Name": "web", "Status": 1, "Type": 2, "EndpointId": 1,
	}))
	mock.On("GET", "/stacks/30/file", RespondJSON(http.StatusOK, map[string]interface{}{
		"StackFileContent": "version: '3'",
	}))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("deployment_type", "standalone")
	_ = d.Set("method", "string")
	_ = d.Set("name", "web")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("stack_file_content", "version: '3'")

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create (existing-stack guard) failed: %v", err)
	}

	if d.Id() != "30" {
		t.Errorf("expected ID %q (reused from existing stack), got %q", "30", d.Id())
	}
	// No create POST must have been sent.
	if mock.FindRequest("POST", "/stacks/create/standalone/string") != nil {
		t.Error("expected NO create POST when stack name already exists")
	}
	// The Update path must have PUT the stack.
	if mock.FindRequest("PUT", "/stacks/30") == nil {
		t.Error("expected PUT /stacks/30 (Update delegation) to be sent")
	}
}

// =============================== READ ===============================

// TestStackRead_HappyPath verifies a successful read hydrates state from the
// stack response and the file-content endpoint.
func TestStackRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/stacks/7", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":         7,
		"Name":       "myapp",
		"Status":     1,
		"Type":       2,
		"SwarmId":    "",
		"EndpointId": 4,
		"Env": []map[string]interface{}{
			{"name": "FOO", "value": "bar"},
		},
	}))
	mock.On("GET", "/stacks/7/file", RespondJSON(http.StatusOK, map[string]interface{}{
		"StackFileContent": "version: '3'\nservices: {}",
	}))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("method", "string")
	d.SetId("7")

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "7" {
		t.Errorf("expected ID to remain %q, got %q", "7", d.Id())
	}
	if got := d.Get("name"); got != "myapp" {
		t.Errorf("name: expected %q, got %v", "myapp", got)
	}
	if got := d.Get("active"); got != true {
		t.Errorf("active: expected true (Status=1), got %v", got)
	}
	if got := d.Get("endpoint_id"); got != 4 {
		t.Errorf("endpoint_id: expected 4, got %v", got)
	}
	if got := d.Get("stack_file_content"); got != "version: '3'\nservices: {}" {
		t.Errorf("stack_file_content: expected populated from file endpoint, got %v", got)
	}
	env := d.Get("env").([]interface{})
	if len(env) != 1 {
		t.Fatalf("env: expected 1 entry, got %d", len(env))
	}
	e0 := env[0].(map[string]interface{})
	if e0["name"] != "FOO" || e0["value"] != "bar" {
		t.Errorf("env[0]: expected FOO=bar, got %v", e0)
	}
}

// TestStackRead_InactiveStatus verifies Status != 1 maps to active=false.
func TestStackRead_InactiveStatus(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/stacks/9", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 9, "Name": "stopped", "Status": 2, "Type": 2, "EndpointId": 1,
	}))
	mock.On("GET", "/stacks/9/file", RespondJSON(http.StatusOK, map[string]interface{}{
		"StackFileContent": "version: '3'",
	}))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("method", "string")
	d.SetId("9")

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("active"); got != false {
		t.Errorf("active: expected false (Status=2), got %v", got)
	}
}

// TestStackRead_404_ClearsID verifies that a 404 on Read removes the resource
// from state (standard Terraform drift-detection).
func TestStackRead_404_ClearsID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/stacks/99", RespondString(
		http.StatusNotFound, "application/json",
		`{"message":"stack not found"}`,
	))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("method", "string")
	d.SetId("99")

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read should swallow 404 and clear ID, got error: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared after 404, got %q", d.Id())
	}
}

// TestStackRead_Repository_PopulatesGitConfig verifies the repository read path:
// no file fetch, and git fields are hydrated from gitConfig.
func TestStackRead_Repository_PopulatesGitConfig(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/stacks/15", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":         15,
		"Name":       "gitapp",
		"Status":     1,
		"Type":       2,
		"EndpointId": 1,
		"gitConfig": map[string]interface{}{
			"URL":            "https://github.com/acme/app.git",
			"ReferenceName":  "refs/heads/prod",
			"ConfigFilePath": "stacks/app.yml",
			"tlsskipVerify":  true,
			"Authentication": map[string]interface{}{"GitCredentialID": 3},
		},
	}))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("method", "repository")
	d.SetId("15")

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	// Repository read must NOT fetch the stack file.
	if mock.FindRequest("GET", "/stacks/15/file") != nil {
		t.Error("did not expect GET /stacks/15/file for repository method")
	}
	if got := d.Get("repository_url"); got != "https://github.com/acme/app.git" {
		t.Errorf("repository_url: expected populated, got %v", got)
	}
	if got := d.Get("repository_reference_name"); got != "refs/heads/prod" {
		t.Errorf("repository_reference_name: expected refs/heads/prod, got %v", got)
	}
	if got := d.Get("file_path_in_repository"); got != "stacks/app.yml" {
		t.Errorf("file_path_in_repository: expected stacks/app.yml, got %v", got)
	}
	if got := d.Get("repository_git_credential_id"); got != 3 {
		t.Errorf("repository_git_credential_id: expected 3, got %v", got)
	}
}

// =============================== UPDATE ===============================

// TestStackUpdate_NoActiveChange_SkipsStartStop documents and pins the
// start/stop behavior under unit testing. The start/stop branch in Update is
// gated on d.HasChange("active"). Build-time TestResourceData carries no diff
// state, so HasChange is always false (same limitation noted in
// resource_webhook_test.go). Therefore Update must NOT issue any
// /stacks/{id}/start or /stacks/{id}/stop request; it proceeds straight to the
// content update PUT.
//
// (The wire format of the start/stop request itself — POST
// /stacks/{id}/{action}?endpointId={n} with no body — is asserted indirectly:
// if a future refactor stops gating on HasChange, this test will fail because
// an unexpected start/stop request would appear.)
func TestStackUpdate_NoActiveChange_SkipsStartStop(t *testing.T) {
	mock := NewMockServer(t)

	// Register start/stop handlers so that, if Update wrongly fires them, the
	// request is recorded (and our assertions below catch it). If they were
	// NOT registered, a stray call would 404 and surface as an Update error
	// instead — either way the test fails, but recording is clearer.
	mock.On("POST", "/stacks/3/stop", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("POST", "/stacks/3/start", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("PUT", "/stacks/3", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 3, "Name": "app"}))
	mock.On("GET", "/stacks/3", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 3, "Name": "app", "Status": 1, "Type": 2, "EndpointId": 1,
	}))
	mock.On("GET", "/stacks/3/file", RespondJSON(http.StatusOK, map[string]interface{}{
		"StackFileContent": "version: '3'",
	}))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	d.SetId("3")
	_ = d.Set("method", "string")
	_ = d.Set("name", "app")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("stack_file_content", "version: '3'")
	_ = d.Set("active", false)

	if err := r.Update(d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// HasChange("active") is false under TestResourceData => no start/stop.
	if mock.FindRequest("POST", "/stacks/3/stop") != nil {
		t.Error("did not expect POST /stacks/3/stop: HasChange(active) is false under TestResourceData")
	}
	if mock.FindRequest("POST", "/stacks/3/start") != nil {
		t.Error("did not expect POST /stacks/3/start: HasChange(active) is false under TestResourceData")
	}
	// The content update PUT must still be sent.
	if mock.FindRequest("PUT", "/stacks/3") == nil {
		t.Error("expected the content update PUT /stacks/3 to be sent")
	}
}

// TestStackUpdate_NonRepositoryContent verifies the standard (non-repository)
// content update PUT carries the stack file content and env, and chains into
// Read.
func TestStackUpdate_NonRepositoryContent(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/stacks/4", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 4, "Name": "app"}))
	mock.On("GET", "/stacks/4", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 4, "Name": "app", "Status": 1, "Type": 2, "EndpointId": 1,
	}))
	mock.On("GET", "/stacks/4/file", RespondJSON(http.StatusOK, map[string]interface{}{
		"StackFileContent": "version: '3.9'",
	}))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	d.SetId("4")
	_ = d.Set("method", "string")
	_ = d.Set("name", "app")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("stack_file_content", "version: '3.9'")

	if err := r.Update(d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/stacks/4")
	if put == nil {
		t.Fatal("expected PUT /stacks/4 (content update) to be sent")
	}
	if !strings.Contains(put.Query, "endpointId=1") {
		t.Errorf("expected update PUT query to carry endpointId=1, got %q", put.Query)
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode update PUT body: %v", err)
	}
	if got := payload["stackFileContent"]; got != "version: '3.9'" {
		t.Errorf("payload.stackFileContent: expected updated content, got %v", got)
	}
	if mock.FindRequest("GET", "/stacks/4") == nil {
		t.Error("expected Update to chain into Read at GET /stacks/4")
	}
}

// TestStackUpdate_Repository_GitRedeploy covers the repository update path:
// POST /stacks/{id}/git (settings) followed by PUT /stacks/{id}/git/redeploy.
// Verifies both requests fire and the redeploy payload carries repository
// fields and the stackName.
func TestStackUpdate_Repository_GitRedeploy(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/stacks/6/git", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("PUT", "/stacks/6/git/redeploy", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/stacks/6", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 6, "Name": "gitapp", "Status": 1, "Type": 2, "EndpointId": 1,
		"gitConfig": map[string]interface{}{
			"URL":           "https://github.com/acme/app.git",
			"ReferenceName": "refs/heads/main",
		},
	}))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	d.SetId("6")
	_ = d.Set("method", "repository")
	_ = d.Set("name", "gitapp")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("repository_url", "https://github.com/acme/app.git")
	_ = d.Set("repository_reference_name", "refs/heads/main")
	_ = d.Set("repository_username", "robot")
	_ = d.Set("repository_password", "secret")

	if err := r.Update(d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	gitPost := mock.FindRequest("POST", "/stacks/6/git")
	if gitPost == nil {
		t.Fatal("expected POST /stacks/6/git (git settings update) to be sent")
	}
	if !strings.Contains(gitPost.Query, "endpointId=1") {
		t.Errorf("expected git settings query to carry endpointId=1, got %q", gitPost.Query)
	}

	redeploy := mock.FindRequest("PUT", "/stacks/6/git/redeploy")
	if redeploy == nil {
		t.Fatal("expected PUT /stacks/6/git/redeploy to be sent")
	}
	var payload map[string]interface{}
	if err := redeploy.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode redeploy PUT body: %v", err)
	}
	if got := payload["stackName"]; got != "gitapp" {
		t.Errorf("redeploy payload.stackName: expected %q, got %v", "gitapp", got)
	}
	if got := payload["repositoryReferenceName"]; got != "refs/heads/main" {
		t.Errorf("redeploy payload.repositoryReferenceName: expected refs/heads/main, got %v", got)
	}
	if got := payload["repositoryUsername"]; got != "robot" {
		t.Errorf("redeploy payload.repositoryUsername: expected robot, got %v", got)
	}

	// Repository update must NOT issue the plain content PUT /stacks/6.
	if mock.FindRequest("PUT", "/stacks/6") != nil {
		t.Error("did not expect plain content PUT /stacks/6 for repository update")
	}
}

// =============================== DELETE ===============================

// TestStackDelete_HappyPath verifies DELETE /stacks/{id} with the endpointId
// query param. A 204 response is treated as success.
func TestStackDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/stacks/5", RespondString(http.StatusNoContent, "", ""))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	d.SetId("5")
	_ = d.Set("endpoint_id", 1)

	if err := r.Delete(d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	del := mock.FindRequest("DELETE", "/stacks/5")
	if del == nil {
		t.Fatal("expected DELETE /stacks/5 to be sent")
	}
	if !strings.Contains(del.Query, "endpointId=1") {
		t.Errorf("expected delete query to carry endpointId=1, got %q", del.Query)
	}
}

// TestStackDelete_404_NoError verifies a 404 on delete is swallowed (the stack
// was already gone).
func TestStackDelete_404_NoError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/stacks/77", RespondString(
		http.StatusNotFound, "application/json",
		`{"message":"stack not found"}`,
	))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	d.SetId("77")
	_ = d.Set("endpoint_id", 1)

	if err := r.Delete(d, mock.Client()); err != nil {
		t.Fatalf("Delete should swallow 404, got error: %v", err)
	}
}
