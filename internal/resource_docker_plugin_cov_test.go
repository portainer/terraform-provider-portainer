package internal

import (
	"net/http"
	"testing"
)

// TestDockerPluginCreate_WithSettings verifies the settings block is encoded
// into the pull body.
func TestDockerPluginCreate_WithSettings(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/docker/plugins/pull", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceDockerPlugin()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("remote", "vieux/sshfs:latest")
	_ = d.Set("name", "sshfs")
	_ = d.Set("settings", []interface{}{
		map[string]interface{}{
			"name":        "DEBUG",
			"description": "enable debug",
			"value":       []interface{}{"1"},
		},
	})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	post := mock.FindRequest("POST", "/endpoints/1/docker/plugins/pull")
	if post == nil {
		t.Fatal("expected pull POST")
	}
	var settings []map[string]interface{}
	if err := post.DecodeJSON(&settings); err != nil {
		t.Fatalf("decode settings body: %v", err)
	}
	if len(settings) != 1 {
		t.Fatalf("expected 1 setting, got %d", len(settings))
	}
	if settings[0]["Name"] != "DEBUG" {
		t.Errorf("setting Name: expected %q, got %v", "DEBUG", settings[0]["Name"])
	}
	if settings[0]["Description"] != "enable debug" {
		t.Errorf("setting Description: expected %q, got %v", "enable debug", settings[0]["Description"])
	}
}

// TestDockerPluginCreate_EnableError verifies that a failed enable after a
// successful pull surfaces an error.
func TestDockerPluginCreate_EnableError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/docker/plugins/pull", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("POST", "/endpoints/1/docker/plugins/sshfs/enable", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"cannot enable"}`,
	))

	r := resourceDockerPlugin()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("remote", "vieux/sshfs:latest")
	_ = d.Set("name", "sshfs")
	_ = d.Set("enable", true)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when enable fails, got nil")
	}
}

// TestDockerPluginRead_WithEnvSettings verifies that env-based settings are
// reconstructed from the inspect payload.
func TestDockerPluginRead_WithEnvSettings(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/plugins/sshfs/json", RespondJSON(http.StatusOK, map[string]interface{}{
		"Enabled": false,
		"Config": map[string]interface{}{
			"Remote": "vieux/sshfs:latest",
			"Settings": map[string]interface{}{
				"Env": []interface{}{"DEBUG=1", "NOEQUALS"},
			},
		},
	}))

	r := resourceDockerPlugin()
	d := r.TestResourceData()
	d.SetId("sshfs")
	_ = d.Set("endpoint_id", 1)

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	settings := d.Get("settings").([]interface{})
	if len(settings) != 2 {
		t.Fatalf("expected 2 reconstructed settings, got %d", len(settings))
	}
	first := settings[0].(map[string]interface{})
	if first["name"] != "DEBUG" {
		t.Errorf("settings[0].name: expected %q, got %v", "DEBUG", first["name"])
	}
}

// TestDockerPluginRead_HTTPError verifies a non-404 4xx/5xx read surfaces an error.
func TestDockerPluginRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/plugins/sshfs/json", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourceDockerPlugin()
	d := r.TestResourceData()
	d.SetId("sshfs")
	_ = d.Set("endpoint_id", 1)

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestDockerPluginDelete_HTTPError verifies a non-2xx/non-404 delete errors.
func TestDockerPluginDelete_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/docker/plugins/sshfs", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourceDockerPlugin()
	d := r.TestResourceData()
	d.SetId("sshfs")
	_ = d.Set("endpoint_id", 1)

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestDockerPluginDelete_404IsSuccess verifies a 404 on delete is success.
func TestDockerPluginDelete_404IsSuccess(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/docker/plugins/sshfs", RespondString(
		http.StatusNotFound, "application/json", `{"message":"gone"}`,
	))

	r := resourceDockerPlugin()
	d := r.TestResourceData()
	d.SetId("sshfs")
	_ = d.Set("endpoint_id", 1)

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete should swallow 404, got error: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}
