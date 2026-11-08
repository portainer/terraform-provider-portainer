package internal

import (
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
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
	// The plugin is installed, so it has to stay in state (tainted) rather
	// than be left behind unmanaged.
	if d.Id() != "sshfs" {
		t.Errorf("expected the installed plugin in state, got ID %q", d.Id())
	}
}

// TestDockerPluginRead_KeepsConfiguredSettings is the regression test for a
// Read that replaced the configured privileges with the plugin's environment
// and the remote with a field Docker does not return. Both force a new
// plugin, so every plan recreated it.
func TestDockerPluginRead_KeepsConfiguredSettings(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/plugins/rclone/json", RespondJSON(http.StatusOK, map[string]interface{}{
		"Name":            "rclone:latest",
		"Enabled":         false,
		"PluginReference": "docker.io/rclone/docker-volume-rclone:latest",
		"Settings": map[string]interface{}{
			"Env": []interface{}{"RCLONE_VERBOSE=0"},
		},
	}))

	r := resourceDockerPlugin()
	d := r.TestResourceData()
	d.SetId("rclone")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("remote", "rclone/docker-volume-rclone")
	_ = d.Set("settings", []interface{}{
		map[string]interface{}{"name": "device", "value": []interface{}{"/dev/fuse"}},
	})

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Get("remote"); got != "rclone/docker-volume-rclone" {
		t.Errorf("remote: expected the configured value, got %v", got)
	}
	settings := d.Get("settings").([]interface{})
	if len(settings) != 1 || settings[0].(map[string]interface{})["name"] != "device" {
		t.Errorf("settings: expected the configured privileges, got %v", settings)
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

// TestDockerPluginCreate_StreamError is the regression test for a Create that
// closed the pull stream unread: Docker cancelled the pull, nothing was
// installed and the apply still succeeded. Docker reports failures inside the
// stream with status 200.
func TestDockerPluginCreate_StreamError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/docker/plugins/pull", RespondString(http.StatusOK, "application/json",
		`{"status":"Downloading"}
{"errorDetail":{"message":"privileges denied"},"error":"privileges denied"}
`))

	r := resourceDockerPlugin()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("remote", "vieux/sshfs")
	_ = d.Set("name", "sshfs")

	err := rcCreate(r, d, mock.Client())
	if err == nil || !strings.Contains(err.Error(), "privileges denied") {
		t.Fatalf("expected the stream error, got %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected no ID after a failed pull, got %q", d.Id())
	}
}

// TestDockerPluginSettingsDescriptionIsInformational covers an imported
// plugin: the settings come from the privileges without a description, and a
// configured description used to force a replacement. Docker compares
// privileges by name and value only.
func TestDockerPluginSettingsDescriptionIsInformational(t *testing.T) {
	r := resourceDockerPlugin()
	suppress := r.Schema["settings"].Elem.(*schema.Resource).Schema["description"].DiffSuppressFunc
	existing := r.TestResourceData()
	existing.SetId("rclone")
	if !suppress("settings.0.description", "", "host network", existing) {
		t.Error("a description change on an existing plugin must not be planned")
	}
	if suppress("settings.0.description", "", "host network", r.TestResourceData()) {
		t.Error("a new plugin keeps the configured description")
	}
}
