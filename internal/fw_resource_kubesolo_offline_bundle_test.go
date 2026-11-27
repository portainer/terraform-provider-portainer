package internal

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
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

// fakeKubeSoloBundles is a stateful stand-in for the offline bundle API: the
// upload stores the raw body, the listing reports it.
type fakeKubeSoloBundles struct {
	mu      sync.Mutex
	bundles map[string]string
	uploads []string
}

func newFakeKubeSoloBundles(m *MockServer, names ...string) *fakeKubeSoloBundles {
	f := &fakeKubeSoloBundles{bundles: map[string]string{}}
	m.On("GET", "/kubesolo/offline-bundles", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		list := make([]map[string]interface{}, 0, 1+len(f.bundles))
		list = append(list, map[string]interface{}{"name": "other-offline.tar.gz", "arch": "arm64", "libc": "musl", "size": 1})
		for name, content := range f.bundles {
			list = append(list, map[string]interface{}{"name": name, "arch": "amd64", "libc": "glibc", "size": len(content)})
		}
		RespondJSON(http.StatusOK, list)(w, r)
	})
	for _, name := range names {
		name := name
		m.On("PUT", "/kubesolo/offline-bundles/"+name, func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			f.mu.Lock()
			defer f.mu.Unlock()
			if r.Header.Get("Content-Type") != "application/octet-stream" {
				RespondString(http.StatusBadRequest, "", "expected an octet-stream body")(w, r)
				return
			}
			f.bundles[name] = string(body)
			f.uploads = append(f.uploads, string(body))
			RespondJSON(http.StatusOK, map[string]interface{}{"name": name, "arch": "amd64", "libc": "glibc", "size": len(body)})(w, r)
		})
		m.On("DELETE", "/kubesolo/offline-bundles/"+name, func(w http.ResponseWriter, _ *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			delete(f.bundles, name)
			w.WriteHeader(http.StatusNoContent)
		})
	}
	return f
}

func kubeSoloBundleHCL(fp string) string {
	return `resource "portainer_kubesolo_offline_bundle" "b" {
  name      = "` + testKubeSoloBundleName + `"
  file_path = "` + fp + `"
}`
}

// TestKubeSoloOfflineBundleResource_Lifecycle: upload, an unchanged plan, a
// re-upload when the tarball is rewritten in place, and import.
func TestKubeSoloOfflineBundleResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeKubeSoloBundles(mock, testKubeSoloBundleName)
	fp := writeTestBundle(t, "abc")

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + kubeSoloBundleHCL(fp),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_kubesolo_offline_bundle.b", "id", testKubeSoloBundleName),
					resource.TestCheckResourceAttr("portainer_kubesolo_offline_bundle.b", "arch", "amd64"),
					resource.TestCheckResourceAttr("portainer_kubesolo_offline_bundle.b", "libc", "glibc"),
					resource.TestCheckResourceAttr("portainer_kubesolo_offline_bundle.b", "size", "3"),
					resource.TestCheckResourceAttr("portainer_kubesolo_offline_bundle.b", "file_sha256", "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"),
				),
			},
			{
				PreConfig: func() {
					if err := os.WriteFile(fp, []byte("abcdef"), 0o600); err != nil {
						t.Fatal(err)
					}
				},
				Config: mockProviderConfig(mock) + kubeSoloBundleHCL(fp),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_kubesolo_offline_bundle.b", plancheck.ResourceActionUpdate)},
				},
				Check: resource.TestCheckResourceAttr("portainer_kubesolo_offline_bundle.b", "size", "6"),
			},
			{
				ResourceName:            "portainer_kubesolo_offline_bundle.b",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"file_path", "file_sha256"},
			},
		},
	})
	if len(fake.uploads) != 2 || fake.uploads[1] != "abcdef" {
		t.Errorf("expected the rewritten tarball to be uploaded again, got %q", fake.uploads)
	}
}

func TestKubeSoloOfflineBundleResource_Errors(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		setup  func(*MockServer)
		config func(t *testing.T) string
		want   string
	}{
		"rejected": {
			setup: func(m *MockServer) {
				m.On("PUT", "/kubesolo/offline-bundles/"+testKubeSoloBundleName, RespondString(http.StatusBadRequest, "application/json", `{"message":"checksum mismatch"}`))
			},
			config: func(t *testing.T) string { return kubeSoloBundleHCL(writeTestBundle(t, "abc")) },
			want:   `checksum mismatch`,
		},
		"unsupported server": {
			setup: func(m *MockServer) {
				m.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{"ServerVersion": "2.45.0", "ServerEdition": "CE"}))
			},
			config: func(t *testing.T) string { return kubeSoloBundleHCL(writeTestBundle(t, "abc")) },
			want:   `portainer_kubesolo_offline_bundle requires Portainer Business Edition 3.0 or later`,
		},
		"missing file": {
			config: func(*testing.T) string { return kubeSoloBundleHCL("/nonexistent/" + testKubeSoloBundleName) },
			want:   `failed to hash file_path`,
		},
		"invalid name": {
			config: func(*testing.T) string {
				return `resource "portainer_kubesolo_offline_bundle" "b" {
  name      = "bundle.tar.gz"
  file_path = "x"
}`
			},
			want: `must be the published tarball name`,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			if tc.setup != nil {
				tc.setup(mock)
			}
			resource.UnitTest(t, resource.TestCase{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Steps: []resource.TestStep{{
					Config:      mockProviderConfig(mock) + tc.config(t),
					ExpectError: edgeErrRe(tc.want),
				}},
			})
		})
	}

	// A valid musl name passes validation.
	if !kubeSoloOfflineBundleName.MatchString("kubesolo-v1.2.3-linux-arm64-musl-offline.tar.gz") {
		t.Error("valid musl name rejected")
	}
}

func TestKubeSoloOfflineBundleResource_GoneOutsideTerraform(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeKubeSoloBundles(mock, testKubeSoloBundleName)
	config := mockProviderConfig(mock) + kubeSoloBundleHCL(writeTestBundle(t, "abc"))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					fake.mu.Lock()
					delete(fake.bundles, testKubeSoloBundleName)
					fake.mu.Unlock()
				},
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
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

func TestKubeSoloOfflineBundle_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeKubeSoloBundles(mock, testKubeSoloBundleName)
	config := mockProviderConfig(mock) + kubeSoloBundleHCL(writeTestBundle(t, "abc"))
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		Steps: []resource.TestStep{
			{ExternalProviders: previousRelease, Config: config},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   config,
				ConfigPlanChecks:         resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}

func TestAccKubeSoloOfflineBundleResource(t *testing.T) {
	if os.Getenv("PORTAINER_ACC_BE") != "1" {
		t.Skip("KubeSolo offline bundles need Portainer Business Edition 3.0 (PORTAINER_ACC_BE=1) and a published KubeSolo tarball")
	}
	t.Skip("needs a KubeSolo release tarball matching the release bundled with the Portainer under test")
}
