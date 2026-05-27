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

	if err := r.Create(d, mock.Client()); err != nil {
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

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if mock.FindRequest("POST", "/endpoints/1/docker/plugins/sshfs/enable") == nil {
		t.Error("expected enable POST to be sent")
	}
}

// TestDockerPluginRead_HappyPath verifies Read decodes plugin state from
// the /json inspect endpoint.
func TestDockerPluginRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/plugins/sshfs/json", RespondJSON(http.StatusOK, map[string]interface{}{
		"Enabled": true,
		"Config": map[string]interface{}{
			"Remote": "vieux/sshfs:latest",
			"Settings": map[string]interface{}{
				"Env": []interface{}{"DEBUG=1"},
			},
		},
	}))

	r := resourceDockerPlugin()
	d := r.TestResourceData()
	d.SetId("sshfs")
	_ = d.Set("endpoint_id", 1)

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Get("enable"); got != true {
		t.Errorf("enable: expected true, got %v", got)
	}
	if got := d.Get("remote"); got != "vieux/sshfs:latest" {
		t.Errorf("remote: expected %q, got %v", "vieux/sshfs:latest", got)
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

	if err := r.Read(d, mock.Client()); err != nil {
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

	if err := r.Delete(d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if mock.FindRequest("DELETE", "/endpoints/1/docker/plugins/sshfs") == nil {
		t.Error("expected DELETE /endpoints/1/docker/plugins/sshfs")
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

	err := r.Create(d, mock.Client())
	if err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}
