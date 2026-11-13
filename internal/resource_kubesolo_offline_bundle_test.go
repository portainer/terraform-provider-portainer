package internal

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

const testKubeSoloBundleName = "kubesolo-v1.2.3-linux-amd64-offline.tar.gz"

// writeTestBundle writes a stand-in tarball and returns its path.
func writeTestBundle(t *testing.T, content string) string {
	t.Helper()
	fp := filepath.Join(t.TempDir(), testKubeSoloBundleName)
	if err := os.WriteFile(fp, []byte(content), 0o600); err != nil {
		t.Fatalf("write tmp: %v", err)
	}
	return fp
}

func kubeSoloBundleListing() http.HandlerFunc {
	return RespondJSON(http.StatusOK, []map[string]interface{}{
		{"name": "other-offline.tar.gz", "arch": "arm64", "libc": "musl", "size": 1},
		{"name": testKubeSoloBundleName, "arch": "amd64", "libc": "glibc", "size": 3},
	})
}

// TestKubeSoloOfflineBundleUpload_HappyPath verifies the tarball is sent as
// the raw octet-stream body and state is filled from the listing.
func TestKubeSoloOfflineBundleUpload_HappyPath(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/kubesolo/offline-bundles/"+testKubeSoloBundleName, RespondJSON(http.StatusOK, map[string]interface{}{
		"name": testKubeSoloBundleName, "arch": "amd64", "libc": "glibc", "size": 3,
	}))
	mock.On("GET", "/kubesolo/offline-bundles", kubeSoloBundleListing())

	r := resourceKubeSoloOfflineBundle()
	d := r.TestResourceData()
	_ = d.Set("name", testKubeSoloBundleName)
	_ = d.Set("file_path", writeTestBundle(t, "abc"))

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != testKubeSoloBundleName {
		t.Errorf("ID: got %q", d.Id())
	}
	if got := d.Get("arch"); got != "amd64" {
		t.Errorf("arch: got %v", got)
	}
	if got := d.Get("size"); got != 3 {
		t.Errorf("size: got %v", got)
	}
	if got := d.Get("file_sha256"); got != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Errorf("file_sha256: got %v", got)
	}

	put := mock.FindRequest("PUT", "/kubesolo/offline-bundles/"+testKubeSoloBundleName)
	if put == nil {
		t.Fatal("expected PUT /kubesolo/offline-bundles/{name}")
	}
	if string(put.Body) != "abc" {
		t.Errorf("body: expected the raw file, got %q", put.Body)
	}
	if got := put.Headers.Get("Content-Type"); got != "application/octet-stream" {
		t.Errorf("Content-Type: got %q", got)
	}
	if put.Headers.Get("X-API-Key") == "" {
		t.Error("expected the API key on the upload")
	}
}

// TestKubeSoloOfflineBundleUpload_Rejected verifies a tarball Portainer
// refuses (wrong release, bad checksum) fails the create.
func TestKubeSoloOfflineBundleUpload_Rejected(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/kubesolo/offline-bundles/"+testKubeSoloBundleName, RespondString(
		http.StatusBadRequest, "application/json", `{"message":"doesn't match the release's SHA256SUMS"}`,
	))

	r := resourceKubeSoloOfflineBundle()
	d := r.TestResourceData()
	_ = d.Set("name", testKubeSoloBundleName)
	_ = d.Set("file_path", writeTestBundle(t, "abc"))

	err := rcCreate(r, d, mock.Client())
	if err == nil || !strings.Contains(err.Error(), "SHA256SUMS") {
		t.Fatalf("expected the server's reason, got %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID, got %q", d.Id())
	}
}

// TestKubeSoloOfflineBundleUpload_UnsupportedServer verifies the 404 of a CE
// server is explained.
func TestKubeSoloOfflineBundleUpload_UnsupportedServer(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{
		"ServerVersion": "2.45.0", "ServerEdition": "CE",
	}))

	r := resourceKubeSoloOfflineBundle()
	d := r.TestResourceData()
	_ = d.Set("name", testKubeSoloBundleName)
	_ = d.Set("file_path", writeTestBundle(t, "abc"))

	err := rcCreate(r, d, mock.Client())
	if err == nil || !strings.Contains(err.Error(), "requires Portainer Business Edition 3.0 or later") {
		t.Fatalf("expected an explained error, got %v", err)
	}
}

// TestKubeSoloOfflineBundleUpload_MissingFile verifies a missing file fails
// before anything is sent.
func TestKubeSoloOfflineBundleUpload_MissingFile(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubeSoloOfflineBundle()
	d := r.TestResourceData()
	_ = d.Set("name", testKubeSoloBundleName)
	_ = d.Set("file_path", "/nonexistent/"+testKubeSoloBundleName)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected an error for a missing file")
	}
	if len(mock.Requests()) != 0 {
		t.Errorf("expected no request, got %d", len(mock.Requests()))
	}
}

// TestKubeSoloOfflineBundleUpload_GetBodyReplays verifies the upload request
// can be replayed, so the retry transport may retry the PUT.
func TestKubeSoloOfflineBundleUpload_GetBodyReplays(t *testing.T) {
	var bodies []string
	mock := NewMockServer(t)
	mock.On("PUT", "/kubesolo/offline-bundles/"+testKubeSoloBundleName, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		if len(bodies) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		RespondJSON(http.StatusOK, map[string]interface{}{"name": testKubeSoloBundleName})(w, r)
	})

	client := mock.Client()
	client.HTTPClient.Transport = &retryTransport{next: http.DefaultTransport, maxRetries: 1}
	prev := retryBaseDelay
	retryBaseDelay = 0
	t.Cleanup(func() { retryBaseDelay = prev })

	if _, err := uploadKubeSoloOfflineBundle(context.Background(), client, testKubeSoloBundleName, writeTestBundle(t, "abc")); err != nil {
		t.Fatalf("upload failed: %v", err)
	}
	if len(bodies) != 2 || bodies[1] != "abc" {
		t.Errorf("expected the full body on the retry, got %q", bodies)
	}
}

// TestKubeSoloOfflineBundleRead_Gone verifies a bundle missing from the
// listing is dropped from state.
func TestKubeSoloOfflineBundleRead_Gone(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubesolo/offline-bundles", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	r := resourceKubeSoloOfflineBundle()
	d := r.TestResourceData()
	d.SetId(testKubeSoloBundleName)

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestKubeSoloOfflineBundleRead_Import verifies an imported bundle gets its
// name and details from the listing.
func TestKubeSoloOfflineBundleRead_Import(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubesolo/offline-bundles", kubeSoloBundleListing())

	r := resourceKubeSoloOfflineBundle()
	d := r.TestResourceData()
	d.SetId(testKubeSoloBundleName)

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("name"); got != testKubeSoloBundleName {
		t.Errorf("name: got %v", got)
	}
	if got := d.Get("libc"); got != "glibc" {
		t.Errorf("libc: got %v", got)
	}
}

// TestKubeSoloOfflineBundleDelete verifies DELETE and that a missing bundle is
// not an error.
func TestKubeSoloOfflineBundleDelete(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("DELETE", "/kubesolo/offline-bundles/"+testKubeSoloBundleName, RespondString(http.StatusNoContent, "", ""))
	mock.On("DELETE", "/kubesolo/offline-bundles/gone-offline.tar.gz", RespondString(http.StatusNotFound, "application/json", `{"message":"gone"}`))

	r := resourceKubeSoloOfflineBundle()
	for _, id := range []string{testKubeSoloBundleName, "gone-offline.tar.gz"} {
		d := r.TestResourceData()
		d.SetId(id)
		if err := rcDelete(r, d, mock.Client()); err != nil {
			t.Fatalf("Delete %s failed: %v", id, err)
		}
	}
}

// TestKubeSoloOfflineBundle_DiffHashAndName covers the plan-time hash that
// re-uploads a rewritten tarball, and the name validation.
func TestKubeSoloOfflineBundle_DiffHashAndName(t *testing.T) {
	r := resourceKubeSoloOfflineBundle()

	cfg := terraform.NewResourceConfigRaw(map[string]interface{}{
		"name":      testKubeSoloBundleName,
		"file_path": writeTestBundle(t, "abc"),
	})
	diff, err := r.Diff(context.Background(), nil, cfg, nil)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	if got := diff.Attributes["file_sha256"]; got == nil || got.New != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Errorf("expected file_sha256 in the diff, got %+v", got)
	}

	missing := terraform.NewResourceConfigRaw(map[string]interface{}{
		"name":      testKubeSoloBundleName,
		"file_path": "/nonexistent/" + testKubeSoloBundleName,
	})
	if _, err := r.Diff(context.Background(), nil, missing, nil); err == nil {
		t.Error("expected a diff error for a missing file")
	}

	validate := r.Schema["name"].ValidateFunc
	if _, errs := validate(testKubeSoloBundleName, "name"); len(errs) != 0 {
		t.Errorf("valid name rejected: %v", errs)
	}
	if _, errs := validate("kubesolo-v1.2.3-linux-arm64-musl-offline.tar.gz", "name"); len(errs) != 0 {
		t.Errorf("valid musl name rejected: %v", errs)
	}
	if _, errs := validate("bundle.tar.gz", "name"); len(errs) == 0 {
		t.Error("expected an invalid name to be rejected")
	}
}
