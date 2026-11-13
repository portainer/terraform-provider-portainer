package internal

import (
	"net/http"
	"strings"
	"testing"
)

// TestDataSourceKubeSoloVersions_Reads covers the version listing, including
// the GitHub error Portainer reports when it falls back to the bundled one.
func TestDataSourceKubeSoloVersions_Reads(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubesolo/versions", RespondJSON(http.StatusOK, map[string]interface{}{
		"bundled":     "v1.2.3",
		"minimum":     "v1.2.0",
		"versions":    []string{"v1.2.4", "v1.2.3"},
		"githubError": "GitHub answered HTTP 502",
	}))

	ds := dataSourceKubeSoloVersions()
	d := ds.TestResourceData()
	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("bundled"); got != "v1.2.3" {
		t.Errorf("bundled: got %v", got)
	}
	if got := d.Get("minimum"); got != "v1.2.0" {
		t.Errorf("minimum: got %v", got)
	}
	if got := d.Get("versions").([]interface{}); len(got) != 2 || got[0] != "v1.2.4" {
		t.Errorf("versions: got %v", got)
	}
	if got := d.Get("github_error"); got != "GitHub answered HTTP 502" {
		t.Errorf("github_error: got %v", got)
	}
	if d.Id() == "" {
		t.Error("expected an ID")
	}
}

// TestDataSourceKubeSoloVersions_UnsupportedServer verifies the 404 of a
// pre-3.0 server is explained.
func TestDataSourceKubeSoloVersions_UnsupportedServer(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{
		"ServerVersion": "2.45.0", "ServerEdition": "BE",
	}))

	ds := dataSourceKubeSoloVersions()
	err := rcRead(ds, ds.TestResourceData(), mock.Client())
	if err == nil || !strings.Contains(err.Error(), "portainer_kubesolo_versions requires Portainer Business Edition 3.0 or later") {
		t.Fatalf("expected an explained error, got %v", err)
	}
}

// TestDataSourceKubeSoloOfflineBundles_Reads covers the bundle listing.
func TestDataSourceKubeSoloOfflineBundles_Reads(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubesolo/offline-bundles", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"name": "kubesolo-v1.2.3-linux-amd64-offline.tar.gz", "arch": "amd64", "libc": "glibc", "size": 237625978},
		{"name": "kubesolo-v1.2.3-linux-arm64-musl-offline.tar.gz", "arch": "arm64", "libc": "musl", "size": 1},
	}))

	ds := dataSourceKubeSoloOfflineBundles()
	d := ds.TestResourceData()
	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	bundles := d.Get("bundles").([]interface{})
	if len(bundles) != 2 {
		t.Fatalf("expected two bundles, got %d", len(bundles))
	}
	first := bundles[0].(map[string]interface{})
	if first["arch"] != "amd64" || first["libc"] != "glibc" || first["size"] != 237625978 {
		t.Errorf("first bundle: got %v", first)
	}
}

// TestDataSourceKubeSoloOfflineBundles_Error verifies a server error fails
// the read.
func TestDataSourceKubeSoloOfflineBundles_Error(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubesolo/offline-bundles", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	ds := dataSourceKubeSoloOfflineBundles()
	if err := rcRead(ds, ds.TestResourceData(), mock.Client()); err == nil {
		t.Fatal("expected an error on 500")
	}
}

// TestDataSourceKubeSoloConfig_Reads covers the configuration read, with the
// configuration kept as JSON.
func TestDataSourceKubeSoloConfig_Reads(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubesolo/environments/7/config", RespondJSON(http.StatusOK, map[string]interface{}{
		"api":     "available",
		"version": "v1.2.2",
		"readAt":  1767225600,
		"config":  map[string]interface{}{"portainerEdgeKey": "secret-key"},
	}))

	ds := dataSourceKubeSoloConfig()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 7)
	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "7" {
		t.Errorf("ID: got %q", d.Id())
	}
	if got := d.Get("api"); got != "available" {
		t.Errorf("api: got %v", got)
	}
	if got := d.Get("version"); got != "v1.2.2" {
		t.Errorf("version: got %v", got)
	}
	if got := d.Get("read_at"); got != 1767225600 {
		t.Errorf("read_at: got %v", got)
	}
	if got := d.Get("config_json"); got != `{"portainerEdgeKey":"secret-key"}` {
		t.Errorf("config_json: got %v", got)
	}
	if !ds.Schema["config_json"].Sensitive {
		t.Error("config_json carries secrets and must be Sensitive")
	}
}

// TestDataSourceKubeSoloConfig_NoConfig verifies an unreachable KubeSolo API
// reads as an empty configuration plus the reason.
func TestDataSourceKubeSoloConfig_NoConfig(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubesolo/environments/7/config", RespondJSON(http.StatusOK, map[string]interface{}{
		"api":    "unreachable",
		"readAt": 1,
		"error":  "connection refused",
	}))

	ds := dataSourceKubeSoloConfig()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 7)
	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("config_json"); got != "" {
		t.Errorf("config_json: expected empty, got %v", got)
	}
	if got := d.Get("error"); got != "connection refused" {
		t.Errorf("error: got %v", got)
	}
}

// TestDataSourceKubeSoloConfig_NotFound verifies a missing environment or
// snapshot fails the read on a server that supports KubeSolo.
func TestDataSourceKubeSoloConfig_NotFound(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{
		"ServerVersion": "3.0.0", "ServerEdition": "BE",
	}))

	ds := dataSourceKubeSoloConfig()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 8)
	err := rcRead(ds, d, mock.Client())
	if err == nil || strings.Contains(err.Error(), "requires Portainer") {
		t.Fatalf("expected a plain not-found error, got %v", err)
	}
}
