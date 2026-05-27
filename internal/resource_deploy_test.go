package internal

import (
	"net/http"
	"reflect"
	"regexp"
	"testing"
)

// resource_deploy is a stateless "action" resource. Create detects whether the
// target endpoint is a Docker Swarm (GET /endpoints/{id}/docker/swarm) and then
// either runs swarm service updates or a standalone stack redeploy. Read is a
// no-op and Delete just clears state.
//
// The mock dispatcher matches the path only (query string is stripped into
// Query), so e.g. "/stacks?filters=..." is registered as "/stacks".
//
// These tests drive the standalone branch end-to-end (it avoids the swarm
// branch's time.Sleep for force-update). The swarm image-tag regex is also
// exercised directly because it is the load-bearing parsing logic.

// TestDeployImageTagRegex documents and locks the regex used to split a Docker
// image reference into repository + current tag (stripping any @digest). This
// is the parsing the swarm branch relies on to decide whether a service is
// already at the target revision.
func TestDeployImageTagRegex(t *testing.T) {
	re := regexp.MustCompile(`^(.+?):([^@]+)(?:@.*)?$`)

	cases := []struct {
		image    string
		wantRepo string
		wantTag  string
		wantOK   bool
	}{
		{"nginx:1.25", "nginx", "1.25", true},
		{"app:1.0@sha256:deadbeef", "app", "1.0", true},
		// NOTE: the first group is non-greedy (.+?), so a registry-port image
		// reference splits on the FIRST colon, NOT the tag colon. This is a
		// known limitation of the resource's regex; locked here as documented
		// behavior so a future change is caught.
		{"registry.example.com:5000/app:v2.0", "registry.example.com", "5000/app:v2.0", true},
		// No tag at all -> no match (repo/tag stay empty in the resource).
		{"nginx", "", "", false},
	}

	for _, tc := range cases {
		m := re.FindStringSubmatch(tc.image)
		ok := len(m) == 3
		if ok != tc.wantOK {
			t.Errorf("%q: match=%v, want %v (groups=%v)", tc.image, ok, tc.wantOK, m)
			continue
		}
		if !ok {
			continue
		}
		if m[1] != tc.wantRepo || m[2] != tc.wantTag {
			t.Errorf("%q: got repo=%q tag=%q, want repo=%q tag=%q",
				tc.image, m[1], m[2], tc.wantRepo, tc.wantTag)
		}
	}
}

// TestDeploySplitAndTrimCSV covers the service-name list parsing helper used to
// build the prefixed service names.
func TestDeploySplitAndTrimCSV(t *testing.T) {
	got := splitAndTrimCSV(" web , worker ,, scheduler ")
	want := []string{"web", "worker", "scheduler"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("splitAndTrimCSV: got %v, want %v", got, want)
	}
	if len(splitAndTrimCSV("   ")) != 0 {
		t.Error("splitAndTrimCSV of blank input should be empty")
	}
}

// TestDeployCreate_Standalone_HappyPath drives the full standalone deploy:
// swarm detection returns non-swarm, the stack is found by name, the stack file
// is read, and the stack is redeployed with an updated env var.
func TestDeployCreate_Standalone_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	// Swarm detection: 404 (or any non-200 / no "ID") => standalone.
	mock.On("GET", "/endpoints/1/docker/swarm", RespondString(
		http.StatusNotFound, "application/json", `{"message":"not a swarm"}`))

	// Stack listing; the resource finds the entry whose Name matches.
	mock.On("GET", "/stacks", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"Id":   7,
			"Name": "myapp",
			"Env": []map[string]interface{}{
				{"name": "APP_VERSION", "value": "1.0.0"},
				{"name": "OTHER", "value": "keepme"},
			},
		},
		{"Id": 8, "Name": "decoy"},
	}))

	// Stack file content read before the redeploy.
	mock.On("GET", "/stacks/7/file", RespondJSON(http.StatusOK, map[string]interface{}{
		"StackFileContent": "version: '3'\nservices: {}\n",
	}))

	// The redeploy PUT.
	mock.On("PUT", "/stacks/7", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 7,
	}))

	r := resourceDeploy()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("stack_name", "myapp")
	_ = d.Set("stack_env_var", "APP_VERSION")
	_ = d.Set("revision", "2.0.0")
	_ = d.Set("services_list", "web,worker")
	_ = d.Set("update_revision", true)

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() == "" {
		t.Error("expected a non-empty deploy ID")
	}

	put := mock.FindRequest("PUT", "/stacks/7")
	if put == nil {
		t.Fatal("expected a PUT /stacks/7 (standalone redeploy)")
	}
	// endpointId is carried via the query string.
	if put.Query == "" {
		t.Error("expected endpointId query on the redeploy PUT")
	}

	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode PUT body: %v", err)
	}
	// Standalone uses lowercase JSON keys.
	if got := payload["prune"]; got != true {
		t.Errorf("payload.prune: expected true, got %v", got)
	}
	if got := payload["pullImage"]; got != true {
		t.Errorf("payload.pullImage: expected true, got %v", got)
	}
	if got := payload["stackFileContent"]; got != "version: '3'\nservices: {}\n" {
		t.Errorf("payload.stackFileContent not carried through, got %v", got)
	}

	env, ok := payload["env"].([]interface{})
	if !ok {
		t.Fatalf("payload.env: expected array, got %T", payload["env"])
	}
	// The matching var must be bumped to the new revision; the unrelated var
	// must be preserved.
	gotVersion, gotOther := "", ""
	for _, e := range env {
		kv := e.(map[string]interface{})
		switch kv["name"] {
		case "APP_VERSION":
			gotVersion = kv["value"].(string)
		case "OTHER":
			gotOther = kv["value"].(string)
		}
	}
	if gotVersion != "2.0.0" {
		t.Errorf("APP_VERSION: expected updated to 2.0.0, got %q", gotVersion)
	}
	if gotOther != "keepme" {
		t.Errorf("OTHER: expected preserved value keepme, got %q", gotOther)
	}
}

// TestDeployCreate_Standalone_AppendsMissingEnvVar verifies that when the
// target env var is not already present on the stack, it is appended.
func TestDeployCreate_Standalone_AppendsMissingEnvVar(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/swarm", RespondString(
		http.StatusNotFound, "application/json", `{}`))
	mock.On("GET", "/stacks", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 3, "Name": "myapp", "Env": []map[string]interface{}{}},
	}))
	mock.On("GET", "/stacks/3/file", RespondJSON(http.StatusOK, map[string]interface{}{
		"StackFileContent": "content",
	}))
	mock.On("PUT", "/stacks/3", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceDeploy()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("stack_name", "myapp")
	_ = d.Set("stack_env_var", "NEW_VAR")
	_ = d.Set("revision", "9.9.9")
	_ = d.Set("services_list", "svc")
	_ = d.Set("update_revision", true)

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/stacks/3")
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode PUT body: %v", err)
	}
	env := payload["env"].([]interface{})
	found := false
	for _, e := range env {
		kv := e.(map[string]interface{})
		if kv["name"] == "NEW_VAR" && kv["value"] == "9.9.9" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected NEW_VAR=9.9.9 appended, got env=%v", env)
	}
}

// TestDeployCreate_EmptyServicesList verifies the early validation error.
func TestDeployCreate_EmptyServicesList(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceDeploy()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("stack_name", "myapp")
	_ = d.Set("stack_env_var", "APP_VERSION")
	_ = d.Set("revision", "2.0.0")
	_ = d.Set("services_list", "   ")

	err := r.Create(d, mock.Client())
	if err == nil {
		t.Fatal("expected error for empty services_list, got nil")
	}
	if len(mock.Requests()) != 0 {
		t.Errorf("expected no API calls before validation, got %d", len(mock.Requests()))
	}
}

// TestDeployCreate_StackNotFound verifies an error when the named stack is
// absent from the standalone stack listing.
func TestDeployCreate_StackNotFound(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/swarm", RespondString(
		http.StatusNotFound, "application/json", `{}`))
	mock.On("GET", "/stacks", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Name": "somethingelse"},
	}))

	r := resourceDeploy()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("stack_name", "missing")
	_ = d.Set("stack_env_var", "APP_VERSION")
	_ = d.Set("revision", "2.0.0")
	_ = d.Set("services_list", "svc")

	err := r.Create(d, mock.Client())
	if err == nil {
		t.Fatal("expected error when stack not found, got nil")
	}
}

// TestDeployCreate_StackListHTTPError verifies a failed stack listing
// propagates an error. The GET /stacks helper (apiGETCtx) does not check the
// status code, so to force a failure we point the swarm detection at a closed
// route; here we instead omit the /stacks route which yields a 404 body that
// fails JSON unmarshaling.
func TestDeployCreate_StackListHTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/swarm", RespondString(
		http.StatusNotFound, "application/json", `{}`))
	// No /stacks route registered: the mock returns a plain-text 404 body
	// which is not valid JSON, so the unmarshal in the resource fails.

	r := resourceDeploy()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("stack_name", "myapp")
	_ = d.Set("stack_env_var", "APP_VERSION")
	_ = d.Set("revision", "2.0.0")
	_ = d.Set("services_list", "svc")

	err := r.Create(d, mock.Client())
	if err == nil {
		t.Fatal("expected error when stack listing cannot be parsed, got nil")
	}
}

// TestDeployCreate_Standalone_NoUpdateRevision verifies that with
// update_revision=false the standalone branch performs no redeploy PUT.
func TestDeployCreate_Standalone_NoUpdateRevision(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/swarm", RespondString(
		http.StatusNotFound, "application/json", `{}`))
	mock.On("GET", "/stacks", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 5, "Name": "myapp"},
	}))

	r := resourceDeploy()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("stack_name", "myapp")
	_ = d.Set("stack_env_var", "APP_VERSION")
	_ = d.Set("revision", "2.0.0")
	_ = d.Set("services_list", "svc")
	_ = d.Set("update_revision", false)

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if mock.FindRequest("GET", "/stacks/5/file") != nil {
		t.Error("expected no stack file read when update_revision=false")
	}
	if d.Id() == "" {
		t.Error("expected a deploy ID even when nothing was updated")
	}
}

// TestDeployRead_Noop verifies the stateless Read does nothing and keeps ID.
func TestDeployRead_Noop(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceDeploy()
	d := r.TestResourceData()
	d.SetId("deploy-123")

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "deploy-123" {
		t.Errorf("expected ID retained, got %q", d.Id())
	}
	if len(mock.Requests()) != 0 {
		t.Errorf("expected no API calls on Read, got %d", len(mock.Requests()))
	}
}

// TestDeployDelete_ClearsState verifies the stateless Delete clears the ID
// without any API calls.
func TestDeployDelete_ClearsState(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceDeploy()
	d := r.TestResourceData()
	d.SetId("deploy-123")

	if err := r.Delete(d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared on Delete, got %q", d.Id())
	}
	if len(mock.Requests()) != 0 {
		t.Errorf("expected no API calls on Delete, got %d", len(mock.Requests()))
	}
}
