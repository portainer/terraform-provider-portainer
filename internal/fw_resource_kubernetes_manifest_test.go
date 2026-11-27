package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// fakeK8sAPI is a stateful stand-in for the Kubernetes API behind Portainer:
// POST to a collection stores the object, GET returns it with the fields the
// API server adds, PATCH applies a JSON merge patch, DELETE removes it.
type fakeK8sAPI struct {
	mu      sync.Mutex
	objects map[string]map[string]interface{}
	// patchStatus / deleteStatus / getStatus, when set, fail those calls.
	patchStatus, deleteStatus, getStatus int
}

func newFakeK8sAPI() *fakeK8sAPI {
	return &fakeK8sAPI{objects: map[string]map[string]interface{}{}}
}

// serve registers the collection and the objects called names in it.
func (f *fakeK8sAPI) serve(m *MockServer, collection string, names ...string) {
	m.On("POST", collection, func(w http.ResponseWriter, r *http.Request) {
		var obj map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&obj); err != nil {
			RespondJSON(http.StatusBadRequest, map[string]interface{}{"message": err.Error()})(w, r)
			return
		}
		meta, _ := obj["metadata"].(map[string]interface{})
		f.mu.Lock()
		f.objects[collection+"/"+fmt.Sprint(meta["name"])] = obj
		f.mu.Unlock()
		RespondJSON(http.StatusCreated, obj)(w, r)
	})
	for _, name := range names {
		objPath := collection + "/" + name
		m.On("GET", objPath, func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			obj, ok := f.objects[objPath]
			status := f.getStatus
			var live map[string]interface{}
			if ok {
				live = k8sFakeLive(obj)
			}
			f.mu.Unlock()
			switch {
			case status != 0:
				RespondJSON(status, map[string]interface{}{"message": "boom"})(w, r)
			case !ok:
				RespondJSON(http.StatusNotFound, map[string]interface{}{"message": "not found"})(w, r)
			default:
				RespondJSON(http.StatusOK, live)(w, r)
			}
		})
		m.On("PATCH", objPath, func(w http.ResponseWriter, r *http.Request) {
			var patch map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&patch)
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.patchStatus != 0 {
				RespondJSON(f.patchStatus, map[string]interface{}{"message": "field is immutable"})(w, r)
				return
			}
			obj, ok := f.objects[objPath]
			if !ok {
				RespondJSON(http.StatusNotFound, map[string]interface{}{"message": "not found"})(w, r)
				return
			}
			f.objects[objPath] = k8sFakeMergePatch(obj, patch)
			RespondJSON(http.StatusOK, f.objects[objPath])(w, r)
		})
		m.On("DELETE", objPath, func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.deleteStatus != 0 {
				RespondJSON(f.deleteStatus, map[string]interface{}{"message": "boom"})(w, r)
				return
			}
			if _, ok := f.objects[objPath]; !ok {
				RespondJSON(http.StatusNotFound, map[string]interface{}{"message": "not found"})(w, r)
				return
			}
			delete(f.objects, objPath)
			RespondJSON(http.StatusOK, map[string]interface{}{})(w, r)
		})
	}
}

func (f *fakeK8sAPI) has(objPath string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.objects[objPath]
	return ok
}

// setLabel changes a label of a stored object, as kubectl would.
func (f *fakeK8sAPI) setLabel(objPath, key, value string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	meta := f.objects[objPath]["metadata"].(map[string]interface{})
	labels, _ := meta["labels"].(map[string]interface{})
	if labels == nil {
		labels = map[string]interface{}{}
		meta["labels"] = labels
	}
	labels[key] = value
}

func (f *fakeK8sAPI) remove(objPath string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.objects, objPath)
}

// k8sFakeLive is obj with what the API server adds: status and metadata.
func k8sFakeLive(obj map[string]interface{}) map[string]interface{} {
	raw, _ := json.Marshal(obj)
	var live map[string]interface{}
	_ = json.Unmarshal(raw, &live)
	if meta, ok := live["metadata"].(map[string]interface{}); ok {
		meta["uid"] = "0b5e"
		meta["resourceVersion"] = "4711"
	}
	live["status"] = map[string]interface{}{}
	return live
}

func k8sFakeMergePatch(obj, patch map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for k, v := range obj {
		out[k] = v
	}
	for k, v := range patch {
		if v == nil {
			delete(out, k)
			continue
		}
		pm, pok := v.(map[string]interface{})
		om, ook := out[k].(map[string]interface{})
		if pok && ook {
			out[k] = k8sFakeMergePatch(om, pm)
			continue
		}
		out[k] = v
	}
	return out
}

// k8sManifestCase is one of the manifest resources, addressed in the
// namespace "default" (where it has one) of endpoint 1.
type k8sManifestCase struct {
	resType    string
	collection string // object "obj" lives at collection + "/obj"
	apiVersion string
	kind       string
	address    string // HCL of the addressing attributes
	id         string // resource ID of "obj"
	createErr  string // createNoun
	deleteErr  string // deleteNoun
}

func (c k8sManifestCase) config(m *MockServer, name, label string) string {
	return mockProviderConfig(m) + fmt.Sprintf(`
resource %q "r" {
  endpoint_id = 1
%s
  manifest = jsonencode({
    apiVersion = %q
    kind       = %q
    metadata   = { name = %q, labels = { app = %q } }
  })
}
`, c.resType, c.address, c.apiVersion, c.kind, name, label)
}

func (c k8sManifestCase) rawConfig(m *MockServer, manifest string) string {
	return mockProviderConfig(m) + fmt.Sprintf(`
resource %q "r" {
  endpoint_id = 1
%s
  manifest = %q
}
`, c.resType, c.address, manifest)
}

const k8sNS = "/endpoints/1/kubernetes/api/v1/namespaces/default"

var k8sManifestCases = []k8sManifestCase{
	{"portainer_kubernetes_application", "/endpoints/1/kubernetes/apis/apps/v1/namespaces/default/deployments", "apps/v1", "Deployment", `  namespace = "default"`, "1:default:obj", "Kubernetes Job", "Job"},
	{"portainer_kubernetes_cronjob", "/endpoints/1/kubernetes/apis/batch/v1/namespaces/default/cronjobs", "batch/v1", "CronJob", `  namespace = "default"`, "1:default:obj", "Kubernetes CronJob", "CronJob"},
	{"portainer_kubernetes_job", "/endpoints/1/kubernetes/apis/batch/v1/namespaces/default/jobs", "batch/v1", "Job", `  namespace = "default"`, "1:default:obj", "Kubernetes Job", "Job"},
	{"portainer_kubernetes_serviceaccounts", k8sNS + "/serviceaccounts", "v1", "ServiceAccount", `  namespace = "default"`, "1:default:obj", "Kubernetes Job", "Job"},
	{"portainer_kubernetes_configmaps", k8sNS + "/configmaps", "v1", "ConfigMap", `  namespace = "default"`, "1:default:obj", "Kubernetes Job", "Job"},
	{"portainer_kubernetes_secret", k8sNS + "/secrets", "v1", "Secret", `  namespace = "default"`, "1:default:obj", "Kubernetes Job", "Job"},
	{"portainer_kubernetes_service", k8sNS + "/services", "v1", "Service", `  namespace = "default"`, "1:default:obj", "Kubernetes Job", "Job"},
	{"portainer_kubernetes_role", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/namespaces/default/roles", "rbac.authorization.k8s.io/v1", "Role", `  namespace = "default"`, "1:default:obj", "Kubernetes Job", "Job"},
	{"portainer_kubernetes_rolebinding", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/namespaces/default/rolebindings", "rbac.authorization.k8s.io/v1", "RoleBinding", `  namespace = "default"`, "1:default:obj", "Kubernetes Job", "Job"},
	{"portainer_kubernetes_clusterrole", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/clusterroles", "rbac.authorization.k8s.io/v1", "ClusterRole", "", "1:obj", "Kubernetes Job", "Job"},
	{"portainer_kubernetes_clusterrolebinding", "/endpoints/1/kubernetes/apis/rbac.authorization.k8s.io/v1/clusterrolebindings", "rbac.authorization.k8s.io/v1", "ClusterRoleBinding", "", "1:obj", "Kubernetes Job", "Job"},
	{"portainer_kubernetes_volume", k8sNS + "/persistentvolumeclaims", "v1", "PersistentVolumeClaim", "  namespace = \"default\"\n  type = \"persistent-volume-claim\"", "1:default:persistent-volume-claim:obj", "Kubernetes volume", "volume"},
	{"portainer_kubernetes_storage", "/endpoints/1/kubernetes/apis/storage.k8s.io/v1/storageclasses", "storage.k8s.io/v1", "StorageClass", "", "1:obj", "Kubernetes Job", "Job"},
}

func k8sManifestCaseFor(t *testing.T, resType string) k8sManifestCase {
	t.Helper()
	for _, c := range k8sManifestCases {
		if c.resType == resType {
			return c
		}
	}
	t.Fatalf("no case for %s", resType)
	return k8sManifestCase{}
}

func k8sNoRequest(m *MockServer, method, path string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		if m.FindRequest(method, path) != nil {
			return fmt.Errorf("unexpected %s %s", method, path)
		}
		return nil
	}
}

func k8sHasRequest(m *MockServer, method, path string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		if m.FindRequest(method, path) == nil {
			return fmt.Errorf("expected %s %s", method, path)
		}
		return nil
	}
}

// TestKubernetesManifestResources_Lifecycle runs every manifest resource
// through real Terraform against the fake: create (payload and composite ID),
// an in-place update by merge patch, import and destroy.
func TestKubernetesManifestResources_Lifecycle(t *testing.T) {
	t.Parallel()
	for _, c := range k8sManifestCases {
		t.Run(c.resType, func(t *testing.T) {
			mock := NewMockServer(t)
			fake := newFakeK8sAPI()
			fake.serve(mock, c.collection, "obj")
			addr := c.resType + ".r"
			objPath := c.collection + "/obj"

			ignore := []string{"manifest"}
			if c.resType == "portainer_kubernetes_application" {
				ignore = append(ignore, "timeouts")
			}
			resource.UnitTest(t, resource.TestCase{
				ProtoV5ProviderFactories: testProtoV5Factories,
				CheckDestroy: func(*terraform.State) error {
					if fake.has(objPath) {
						return fmt.Errorf("%s still exists", objPath)
					}
					return nil
				},
				Steps: []resource.TestStep{
					{
						Config: c.config(mock, "obj", "a"),
						Check: resource.ComposeTestCheckFunc(
							resource.TestCheckResourceAttr(addr, "id", c.id),
							resource.TestCheckResourceAttr(addr, "endpoint_id", "1"),
							func(*terraform.State) error {
								post := mock.FindRequest("POST", c.collection)
								if post == nil {
									return fmt.Errorf("expected a POST to %s", c.collection)
								}
								var payload map[string]interface{}
								if err := post.DecodeJSON(&payload); err != nil {
									return err
								}
								if payload["kind"] != c.kind {
									return fmt.Errorf("payload.kind: expected %q, got %v", c.kind, payload["kind"])
								}
								return nil
							},
						),
					},
					{
						Config: c.config(mock, "obj", "b"),
						ConfigPlanChecks: resource.ConfigPlanChecks{
							PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate)},
						},
						Check: resource.ComposeTestCheckFunc(
							resource.TestCheckResourceAttr(addr, "id", c.id),
							k8sHasRequest(mock, "PATCH", objPath),
							k8sNoRequest(mock, "DELETE", objPath),
						),
					},
					{
						ResourceName:            addr,
						ImportState:             true,
						ImportStateVerify:       true,
						ImportStateVerifyIgnore: ignore,
					},
				},
			})
		})
	}
}

// TestKubernetesManifestResources_Errors covers the create errors and their
// texts, kept from the SDKv2 resources, for every manifest resource.
func TestKubernetesManifestResources_Errors(t *testing.T) {
	t.Parallel()
	for _, c := range k8sManifestCases {
		t.Run(c.resType, func(t *testing.T) {
			mock := NewMockServer(t)
			mock.On("POST", c.collection, RespondString(http.StatusBadRequest, "application/json", `{"message":"bad request"}`))
			resource.UnitTest(t, resource.TestCase{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Steps: []resource.TestStep{
					{Config: c.rawConfig(mock, "[unterminated"), ExpectError: regexp.MustCompile(`manifest must be valid JSON or YAML`)},
					{Config: c.rawConfig(mock, `{"kind":"X"}`), ExpectError: regexp.MustCompile(`missing metadata in manifest`)},
					{Config: c.rawConfig(mock, `{"kind":"X","metadata":{}}`), ExpectError: regexp.MustCompile(`missing metadata.name in manifest`)},
					{Config: c.config(mock, "obj", "a"), ExpectError: regexp.MustCompile(`failed to create ` + c.createErr + `:`)},
				},
			})
		})
	}
}

// TestKubernetesManifestResource_DeleteError: a failed DELETE is reported and
// the resource stays in state.
func TestKubernetesManifestResource_DeleteError(t *testing.T) {
	t.Parallel()
	c := k8sManifestCaseFor(t, "portainer_kubernetes_cronjob")
	mock := NewMockServer(t)
	fake := newFakeK8sAPI()
	fake.serve(mock, c.collection, "obj")
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: c.config(mock, "obj", "a")},
			{
				PreConfig:   func() { fake.mu.Lock(); fake.deleteStatus = http.StatusInternalServerError; fake.mu.Unlock() },
				Config:      c.config(mock, "obj", "a"),
				Destroy:     true,
				ExpectError: regexp.MustCompile(`failed to delete CronJob`),
			},
			{
				PreConfig: func() { fake.mu.Lock(); fake.deleteStatus = 0; fake.mu.Unlock() },
				Config:    c.config(mock, "obj", "a"),
			},
		},
	})
}

// TestKubernetesManifestResource_DeleteNotFoundIsSuccess: a DELETE answered
// with 404 counts as deleted.
func TestKubernetesManifestResource_DeleteNotFoundIsSuccess(t *testing.T) {
	t.Parallel()
	for _, c := range k8sManifestCases {
		mock := NewMockServer(t)
		mock.On("DELETE", c.collection+"/obj", RespondString(http.StatusNotFound, "application/json", `{"message":"nf"}`))
		r := &k8sManifestResource{kind: k8sManifestKindOf(t, c.resType), client: mock.Client()}
		if err := r.deleteObject(context.Background(), c.id, time.Minute); err != nil {
			t.Errorf("%s: expected 404 delete to succeed, got %v", c.resType, err)
		}
		if mock.FindRequest("DELETE", c.collection+"/obj") == nil {
			t.Errorf("%s: expected a DELETE of %s", c.resType, c.collection+"/obj")
		}
	}
}

func k8sManifestKindOf(t *testing.T, resType string) k8sManifestKind {
	t.Helper()
	r := newFrameworkResourceByType(resType)
	if r == nil {
		t.Fatalf("no framework resource %s", resType)
	}
	return r.kind
}

func newFrameworkResourceByType(resType string) *k8sManifestResource {
	for _, k := range []k8sManifestKind{
		k8sKindApplication, k8sKindCronJob, k8sKindJob, k8sKindServiceAccounts, k8sKindConfigMaps,
		k8sKindSecret, k8sKindService, k8sKindRole, k8sKindRoleBinding, k8sKindClusterRole,
		k8sKindClusterRoleBinding, k8sKindVolume, k8sKindStorage,
	} {
		if "portainer_"+k.typeName == resType {
			return &k8sManifestResource{kind: k}
		}
	}
	return nil
}

// TestKubernetesManifestResource_Drift: an authored field changed outside
// Terraform, or the object deleted, plans a change.
func TestKubernetesManifestResource_Drift(t *testing.T) {
	t.Parallel()
	c := k8sManifestCaseFor(t, "portainer_kubernetes_configmaps")
	mock := NewMockServer(t)
	fake := newFakeK8sAPI()
	fake.serve(mock, c.collection, "obj")
	objPath := c.collection + "/obj"
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: c.config(mock, "obj", "a")},
			{
				// Fields the server adds (uid, status, ...) are not drift.
				Config: c.config(mock, "obj", "a"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				PreConfig: func() { fake.setLabel(objPath, "app", "changed") },
				Config:    c.config(mock, "obj", "a"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(c.resType+".r", plancheck.ResourceActionUpdate)},
				},
				Check: k8sHasRequest(mock, "PATCH", objPath),
			},
			{
				PreConfig:          func() { fake.remove(objPath) },
				Config:             c.config(mock, "obj", "a"),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestKubernetesManifestResource_ReadError: an error other than 404 while
// reading is reported.
func TestKubernetesManifestResource_ReadError(t *testing.T) {
	t.Parallel()
	c := k8sManifestCaseFor(t, "portainer_kubernetes_application")
	mock := NewMockServer(t)
	fake := newFakeK8sAPI()
	fake.serve(mock, c.collection, "obj")
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: c.config(mock, "obj", "a")},
			{
				PreConfig:   func() { fake.mu.Lock(); fake.getStatus = http.StatusInternalServerError; fake.mu.Unlock() },
				Config:      c.config(mock, "obj", "a"),
				ExpectError: regexp.MustCompile(`failed to read deployment obj`),
			},
			{
				PreConfig: func() { fake.mu.Lock(); fake.getStatus = 0; fake.mu.Unlock() },
				Config:    c.config(mock, "obj", "a"),
			},
		},
	})
}

// TestKubernetesManifestResource_Recreate: a renamed object, a rejected
// in-place change (422) and a changed namespace are deleted and created again
// in an update, as the SDKv2 resources did.
func TestKubernetesManifestResource_Recreate(t *testing.T) {
	k8sDeletePollInterval = time.Millisecond
	t.Cleanup(func() { k8sDeletePollInterval = time.Second })

	c := k8sManifestCaseFor(t, "portainer_kubernetes_service")
	mock := NewMockServer(t)
	fake := newFakeK8sAPI()
	fake.serve(mock, c.collection, "obj", "obj2")
	other := "/endpoints/1/kubernetes/api/v1/namespaces/other/services"
	fake.serve(mock, other, "obj2")
	addr := c.resType + ".r"
	inOther := c
	inOther.address = `  namespace = "other"`

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: c.config(mock, "obj", "a")},
			{
				Config: c.config(mock, "obj2", "a"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "id", "1:default:obj2"),
					k8sHasRequest(mock, "DELETE", c.collection+"/obj"),
					func(*terraform.State) error {
						if fake.has(c.collection+"/obj") || !fake.has(c.collection+"/obj2") {
							return fmt.Errorf("expected obj replaced by obj2")
						}
						return nil
					},
				),
			},
			{
				PreConfig: func() { fake.mu.Lock(); fake.patchStatus = http.StatusUnprocessableEntity; fake.mu.Unlock() },
				Config:    c.config(mock, "obj2", "b"),
				Check: resource.ComposeTestCheckFunc(
					k8sHasRequest(mock, "PATCH", c.collection+"/obj2"),
					k8sHasRequest(mock, "DELETE", c.collection+"/obj2"),
					resource.TestCheckResourceAttr(addr, "id", "1:default:obj2"),
				),
			},
			{
				PreConfig: func() { fake.mu.Lock(); fake.patchStatus = 0; fake.mu.Unlock() },
				Config:    inOther.config(mock, "obj2", "b"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "id", "1:other:obj2"),
					func(*terraform.State) error {
						if fake.has(c.collection+"/obj2") || !fake.has(other+"/obj2") {
							return fmt.Errorf("expected the service moved to namespace other")
						}
						return nil
					},
				),
			},
		},
	})
}

// TestKubernetesManifestResource_ImportInvalidID: Read rejects an ID it
// cannot parse, with the expected format.
func TestKubernetesManifestResource_ImportInvalidID(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"portainer_kubernetes_application": `expected 'endpointID:namespace:name'`,
		"portainer_kubernetes_clusterrole": `expected 'endpointID:name'`,
		"portainer_kubernetes_volume":      `expected 'endpointID:namespace:type:name'`,
	}
	for resType, want := range cases {
		t.Run(resType, func(t *testing.T) {
			c := k8sManifestCaseFor(t, resType)
			mock := NewMockServer(t)
			resource.UnitTest(t, resource.TestCase{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Steps: []resource.TestStep{{
					Config:        c.config(mock, "obj", "a"),
					ResourceName:  resType + ".r",
					ImportState:   true,
					ImportStateId: "only-one-part",
					ExpectError:   regexp.MustCompile(`invalid ID format, ` + want),
				}},
			})
		})
	}
}

// TestKubernetesVolume_ClusterScopedWithoutNamespace: a persistent volume
// leaves namespace unset; it stays null and plans no change.
func TestKubernetesVolume_ClusterScopedWithoutNamespace(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeK8sAPI()
	fake.serve(mock, "/endpoints/1/kubernetes/api/v1/persistentvolumes", "pv1")
	config := mockProviderConfig(mock) + `
resource "portainer_kubernetes_volume" "pv" {
  endpoint_id = 1
  type        = "persistent-volume"
  manifest    = jsonencode({ apiVersion = "v1", kind = "PersistentVolume", metadata = { name = "pv1" } })
}
`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_kubernetes_volume.pv", "id", "1::persistent-volume:pv1"),
					resource.TestCheckNoResourceAttr("portainer_kubernetes_volume.pv", "namespace"),
				),
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:            "portainer_kubernetes_volume.pv",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"manifest"},
			},
		},
	})
}

func TestKubernetesVolume_UnsupportedTypeRejected(t *testing.T) {
	t.Parallel()
	c := k8sManifestCaseFor(t, "portainer_kubernetes_volume")
	c.address = `  type = "nfs"`
	mock := NewMockServer(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      c.config(mock, "obj", "a"),
			ExpectError: regexp.MustCompile(`must be one of`),
		}},
	})
	if _, err := volumeAPIURL("http://x", 1, "", "nfs", false); err == nil || err.Error() != "unsupported volume type: nfs" {
		t.Errorf("volumeAPIURL: expected the unsupported type error, got %v", err)
	}
}

func TestVolumeAPIURL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		volType, ns, name, want string
	}{
		{"persistent-volume-claim", "default", "", "http://x/endpoints/1/kubernetes/api/v1/namespaces/default/persistentvolumeclaims"},
		{"persistent-volume-claim", "default", "c", "http://x/endpoints/1/kubernetes/api/v1/namespaces/default/persistentvolumeclaims/c"},
		{"persistent-volume", "", "", "http://x/endpoints/1/kubernetes/api/v1/persistentvolumes"},
		{"persistent-volume", "", "p", "http://x/endpoints/1/kubernetes/api/v1/persistentvolumes/p"},
		{"volume-attachment", "", "", "http://x/endpoints/1/kubernetes/apis/storage.k8s.io/v1/volumeattachments"},
		{"volume-attachment", "", "a", "http://x/endpoints/1/kubernetes/apis/storage.k8s.io/v1/volumeattachments/a"},
	}
	for _, tc := range cases {
		var (
			got string
			err error
		)
		if tc.name == "" {
			got, err = volumeAPIURL("http://x", 1, tc.ns, tc.volType, false)
		} else {
			got, err = volumeAPIURL("http://x", 1, tc.ns, tc.volType, true, tc.name)
		}
		if err != nil || got != tc.want {
			t.Errorf("volumeAPIURL(%s, %q): got %q, %v; want %q", tc.volType, tc.name, got, err, tc.want)
		}
	}
}

// TestK8sManifestParseID keeps the ID formats of the SDKv2 parsers: a
// malformed ID gives zero values.
func TestK8sManifestParseID(t *testing.T) {
	t.Parallel()
	type parts struct {
		endpointID               int
		namespace, volType, name string
	}
	cases := []struct {
		scope k8sManifestScope
		id    string
		want  parts
	}{
		{k8sNamespaced, "1:default:my-secret", parts{1, "default", "", "my-secret"}},
		{k8sNamespaced, "5:kube-system:tls-cert", parts{5, "kube-system", "", "tls-cert"}},
		{k8sNamespaced, "7:cron:name:with:colons", parts{7, "cron", "", "name:with:colons"}},
		{k8sNamespaced, "1:default", parts{}},
		{k8sNamespaced, "1", parts{}},
		{k8sNamespaced, "", parts{}},
		{k8sNamespaced, "abc:ns:n", parts{0, "ns", "", "n"}},
		{k8sClusterScoped, "5:my-role", parts{5, "", "", "my-role"}},
		{k8sClusterScoped, "12345:admin-role", parts{12345, "", "", "admin-role"}},
		{k8sClusterScoped, "0:role-name", parts{0, "", "", "role-name"}},
		{k8sClusterScoped, "3:a:b", parts{3, "", "", "a"}},
		{k8sClusterScoped, "5", parts{}},
		{k8sClusterScoped, "", parts{}},
		{k8sVolumeScoped, "1:default:persistent-volume-claim:my-pvc", parts{1, "default", "persistent-volume-claim", "my-pvc"}},
		{k8sVolumeScoped, "2::persistent-volume:pv", parts{2, "", "persistent-volume", "pv"}},
		{k8sVolumeScoped, "1:default:pvc", parts{}},
		{k8sVolumeScoped, "", parts{}},
	}
	for _, tc := range cases {
		var got parts
		got.endpointID, got.namespace, got.volType, got.name = k8sManifestParseID(tc.scope, tc.id)
		if got != tc.want {
			t.Errorf("k8sManifestParseID(%d, %q) = %+v, want %+v", tc.scope, tc.id, got, tc.want)
		}
	}
}

// TestKubernetesManifestResources_UpgradeFromV300: state written by v3.0.0
// (SDKv2) for every manifest resource - with and without the optional
// namespace of portainer_kubernetes_volume and the timeouts block of
// portainer_kubernetes_application - plans no change with this build.
func TestKubernetesManifestResources_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeK8sAPI()
	var config strings.Builder
	config.WriteString(mockProviderConfig(mock))
	for i, c := range k8sManifestCases {
		fake.serve(mock, c.collection, "obj")
		body := strings.TrimPrefix(c.config(mock, "obj", "a"), mockProviderConfig(mock))
		config.WriteString(strings.Replace(body, `"r"`, fmt.Sprintf(`"r%d"`, i), 1))
	}
	fake.serve(mock, "/endpoints/1/kubernetes/api/v1/persistentvolumes", "pv1")
	fake.serve(mock, "/endpoints/1/kubernetes/apis/apps/v1/namespaces/timed/deployments", "app")
	config.WriteString(`
resource "portainer_kubernetes_volume" "pv" {
  endpoint_id = 1
  type        = "persistent-volume"
  manifest    = "kind: PersistentVolume\nmetadata:\n  name: pv1\n"
}

resource "portainer_kubernetes_application" "timed" {
  endpoint_id = 1
  namespace   = "timed"
  manifest    = <<-EOT
    # authored YAML stays as written
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: app
    spec:
      replicas: 2
  EOT
  timeouts {
    create = "3m"
    delete = "1m"
  }
}
`)

	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		Steps: []resource.TestStep{
			{ExternalProviders: previousRelease, Config: config.String()},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   config.String(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TestAccKubernetesConfigMap_Lifecycle runs a ConfigMap through create, an
// in-place update and import on a real Kubernetes environment.
func TestAccKubernetesConfigMap_Lifecycle(t *testing.T) {
	envID := os.Getenv("PORTAINER_ACC_K8S_ENV_ID")
	if envID == "" {
		t.Skip("PORTAINER_ACC_K8S_ENV_ID must name a Kubernetes environment for this acceptance test")
	}
	config := func(value string) string {
		return accProvider() + fmt.Sprintf(`
resource "portainer_kubernetes_configmaps" "cm" {
  endpoint_id = %s
  namespace   = "default"
  manifest = jsonencode({
    apiVersion = "v1"
    kind       = "ConfigMap"
    metadata   = { name = "tf-acc-configmap" }
    data       = { key = %q }
  })
}
`, envID, value)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config("one")},
			{
				Config: config("two"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_kubernetes_configmaps.cm", plancheck.ResourceActionUpdate)},
				},
			},
			{
				ResourceName:            "portainer_kubernetes_configmaps.cm",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"manifest"},
			},
		},
	})
}
