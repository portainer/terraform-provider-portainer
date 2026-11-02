package internal

import (
	"net/http"
	"testing"
)

// TestDockerPluginCreate_HappyPath verifies the pull POST is sent and ID is
// set to the plugin name.
func TestDockerPluginCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/docker/plugins/pull", RespondString(
		http.StatusOK, "application/json",
		`{}`,
	))

	r := resourceDockerPlugin()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("remote", "vieux/sshfs:latest")
	_ = d.Set("name", "sshfs")
	// schema Default isn't materialized in TestResourceData; set explicitly.
	_ = d.Set("registry_auth", "e30=")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "sshfs" {
		t.Errorf("expected ID %q, got %q", "sshfs", d.Id())
	}

	post := mock.FindRequest("POST", "/endpoints/1/docker/plugins/pull")
	if post == nil {
		t.Fatal("expected a POST to /endpoints/1/docker/plugins/pull")
	}
	if post.Query == "" {
		t.Error("expected query string containing remote and name")
	}
	if got := post.Headers.Get("X-Registry-Auth"); got != "e30=" {
		t.Errorf("X-Registry-Auth: expected %q, got %q", "e30=", got)
	}
}

// TestDockerPluginCreate_EnableTrue verifies that enable=true triggers the
// follow-up enable POST.
func TestDockerPluginCreate_EnableTrue(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/docker/plugins/pull", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("POST", "/endpoints/1/docker/plugins/sshfs/enable", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceDockerPlugin()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("remote", "vieux/sshfs:latest")
	_ = d.Set("name", "sshfs")
	_ = d.Set("enable", true)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	enable := mock.FindRequest("POST", "/endpoints/1/docker/plugins/sshfs/enable")
	if enable == nil {
		t.Fatal("expected enable POST to be sent")
	}
	// Docker rejects the enable call without a timeout.
	if enable.Query != "timeout=0" {
		t.Errorf("expected timeout=0, got %q", enable.Query)
	}
}

// TestDockerPluginRead_HappyPath verifies Read decodes plugin state from
// the /json inspect endpoint.
func TestDockerPluginRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/plugins/sshfs/json", RespondJSON(http.StatusOK, map[string]interface{}{
		"Enabled":         true,
		"PluginReference": "docker.io/vieux/sshfs:latest",
	}))
	mock.On("GET", "/endpoints/1/docker/plugins/privileges", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Name": "network", "Description": "permissions to access a network", "Value": []string{"host"}},
	}))

	r := resourceDockerPlugin()
	d := r.TestResourceData()
	d.SetId("sshfs")
	_ = d.Set("endpoint_id", 1)

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Get("enable"); got != true {
		t.Errorf("enable: expected true, got %v", got)
	}
	// An import has no remote yet; it comes from the plugin reference, and
	// the settings from the privileges the plugin requires.
	if got := d.Get("remote"); got != "vieux/sshfs" {
		t.Errorf("remote: expected %q, got %v", "vieux/sshfs", got)
	}
	if req := mock.FindRequest("GET", "/endpoints/1/docker/plugins/privileges"); req == nil || req.Query != "remote=vieux%2Fsshfs" {
		t.Errorf("expected the privileges of vieux/sshfs to be read, got %+v", req)
	}
	settings := d.Get("settings").([]interface{})
	if len(settings) != 1 || settings[0].(map[string]interface{})["name"] != "network" {
		t.Errorf("settings: expected the network privilege, got %v", settings)
	}
}

// TestDockerPluginRead_404ClearsID verifies that a 404 on Read clears the ID.
func TestDockerPluginRead_404ClearsID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/plugins/missing/json", RespondString(
		http.StatusNotFound, "application/json",
		`{"message":"not found"}`,
	))

	r := resourceDockerPlugin()
	d := r.TestResourceData()
	d.SetId("missing")
	_ = d.Set("endpoint_id", 1)

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read should swallow 404, got error: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestDockerPluginDelete_HappyPath verifies DELETE is sent and ID is cleared.
func TestDockerPluginDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/docker/plugins/sshfs", RespondString(http.StatusNoContent, "", ""))

	r := resourceDockerPlugin()
	d := r.TestResourceData()
	d.SetId("sshfs")
	_ = d.Set("endpoint_id", 1)

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	del := mock.FindRequest("DELETE", "/endpoints/1/docker/plugins/sshfs")
	if del == nil {
		t.Fatal("expected DELETE /endpoints/1/docker/plugins/sshfs")
	}
	// Docker refuses to remove an enabled plugin without force.
	if del.Query != "force=true" {
		t.Errorf("expected force=true, got %q", del.Query)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestDockerPluginCreate_HTTPError verifies that a 5xx response is surfaced.
func TestDockerPluginCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/docker/plugins/pull", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"pull failed"}`,
	))

	r := resourceDockerPlugin()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("remote", "bad/plugin:tag")
	_ = d.Set("name", "bad")

	err := rcCreate(r, d, mock.Client())
	if err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}
