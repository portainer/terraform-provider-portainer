package internal

import (
	"fmt"
	"net/http"
	"regexp"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// fakeScopedKubeconfigs issues kubeconfig 5 on environment 2.
type fakeScopedKubeconfigs struct {
	store       *a2Store
	deleteState atomic.Int32 // HTTP status of the next DELETE, 0 = 204
}

func newFakeScopedKubeconfigs(mock *MockServer) *fakeScopedKubeconfigs {
	f := &fakeScopedKubeconfigs{store: newA2Store()}
	mock.On("POST", "/kubernetes/2/scoped_kubeconfig", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		a2Decode(r, &body)
		namespaces, _ := body["namespaces"].([]interface{})
		if len(namespaces) == 0 {
			namespaces = []interface{}{"default", "team-a"}
		}
		expiry, _ := body["expirySeconds"].(float64)
		f.store.put("5", map[string]interface{}{
			"id": 5, "endpointId": 2, "description": body["description"], "namespaces": namespaces,
			"includeSecrets": body["includeSecrets"], "serviceAccount": "portainer-sa-5", "active": true,
			"createdAt": 1000, "expiresAt": 1000 + int(expiry),
		})
		RespondJSON(http.StatusCreated, map[string]interface{}{
			"id": 5, "kubeconfig": "apiVersion: v1\nkind: Config\n", "serviceAccount": "portainer-sa-5",
			"namespaces": namespaces, "expiresAt": 1000 + int(expiry),
		})(w, r)
	})
	f.store.serve(mock, "/kubernetes/2/scoped_kubeconfig/5", "5")
	mock.On("DELETE", "/kubernetes/2/scoped_kubeconfig/5", func(w http.ResponseWriter, r *http.Request) {
		f.store.del("5")
		if status := f.deleteState.Swap(0); status != 0 {
			RespondString(int(status), "", "could not remove the service account")(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	return f
}

const scopedKubeconfigConfig = `
resource "portainer_kubernetes_scoped_kubeconfig" "k" {
  environment_id = 2
  expiry_seconds = 3600
  %s
}
`

// TestKubernetesScopedKubeconfig_Lifecycle: the issue payload (namespaces
// left out when unset), the credential kept in state from the issuing call,
// a kubeconfig that expired dropped on refresh, and import filling the
// arguments the kubeconfig was issued with.
func TestKubernetesScopedKubeconfig_Lifecycle(t *testing.T) {
	mock := NewMockServer(t)
	fake := newFakeScopedKubeconfigs(mock)
	full := mockProviderConfig(mock) + fmt.Sprintf(scopedKubeconfigConfig, `
  namespaces      = ["team-a"]
  include_secrets = true
  description     = "ci"
`)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: full,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_kubernetes_scoped_kubeconfig.k", "id", "2:5"),
					resource.TestCheckResourceAttr("portainer_kubernetes_scoped_kubeconfig.k", "kubeconfig", "apiVersion: v1\nkind: Config\n"),
					resource.TestCheckResourceAttr("portainer_kubernetes_scoped_kubeconfig.k", "created_at", "1000"),
					resource.TestCheckResourceAttr("portainer_kubernetes_scoped_kubeconfig.k", "expires_at", "4600"),
					resource.TestCheckResourceAttr("portainer_kubernetes_scoped_kubeconfig.k", "active", "true"),
					a2Check(func() error {
						var body map[string]interface{}
						a2LastBody(t, mock, "POST", "/kubernetes/2/scoped_kubeconfig", &body)
						if body["expirySeconds"] != float64(3600) || body["includeSecrets"] != true || body["description"] != "ci" {
							return fmt.Errorf("payload: %v", body)
						}
						if ns, _ := body["namespaces"].([]interface{}); len(ns) != 1 || ns[0] != "team-a" {
							return fmt.Errorf("payload.namespaces: %v", body["namespaces"])
						}
						return nil
					}),
				),
			},
			{
				ResourceName:            "portainer_kubernetes_scoped_kubeconfig.k",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"kubeconfig"},
			},
			{
				PreConfig: func() {
					v, _ := fake.store.get("5")
					v.(map[string]interface{})["active"] = false
					fake.store.put("5", v)
				},
				Config:             full,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestKubernetesScopedKubeconfig_NamespacesOmittedAndDestroy(t *testing.T) {
	mock := NewMockServer(t)
	fake := newFakeScopedKubeconfigs(mock)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(scopedKubeconfigConfig, ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("portainer_kubernetes_scoped_kubeconfig.k", "namespaces"),
					resource.TestCheckNoResourceAttr("portainer_kubernetes_scoped_kubeconfig.k", "description"),
					resource.TestCheckResourceAttr("portainer_kubernetes_scoped_kubeconfig.k", "covered_namespaces.#", "2"),
					a2Check(func() error {
						var body map[string]interface{}
						a2LastBody(t, mock, "POST", "/kubernetes/2/scoped_kubeconfig", &body)
						if _, ok := body["namespaces"]; ok {
							return fmt.Errorf("namespaces must be left out, got %v", body["namespaces"])
						}
						if body["description"] != "" || body["includeSecrets"] != false {
							return fmt.Errorf("payload: %v", body)
						}
						return nil
					}),
				),
			},
			{
				// A 500 on revoke: the credential is gone but its objects are
				// not, which is a warning, not a failed destroy.
				PreConfig: func() { fake.deleteState.Store(http.StatusInternalServerError) },
				Config:    mockProviderConfig(mock) + fmt.Sprintf(scopedKubeconfigConfig, ""),
				Destroy:   true,
			},
		},
	})
}

func TestKubernetesScopedKubeconfig_Errors(t *testing.T) {
	old := NewMockServer(t)
	old.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{"ServerVersion": "2.33.0", "ServerEdition": "CE"}))
	noID := NewMockServer(t)
	noID.On("POST", "/kubernetes/2/scoped_kubeconfig", RespondJSON(http.StatusCreated, map[string]interface{}{}))
	unknown := NewMockServer(t)
	unknown.On("GET", "/kubernetes/2/scoped_kubeconfig/5", RespondString(http.StatusNotFound, "", "not found"))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config:      mockProviderConfig(old) + fmt.Sprintf(scopedKubeconfigConfig, ""),
				ExpectError: regexp.MustCompile(`portainer_kubernetes_scoped_kubeconfig\s+requires`),
			},
			{
				Config:      mockProviderConfig(noID) + fmt.Sprintf(scopedKubeconfigConfig, ""),
				ExpectError: regexp.MustCompile(`Portainer returned no identifier`),
			},
			{
				Config:      mockProviderConfig(noID) + `resource "portainer_kubernetes_scoped_kubeconfig" "k" {` + "\n environment_id = 2\n expiry_seconds = 604801\n}",
				ExpectError: regexp.MustCompile(`between 1 and 604800`),
			},
			{
				Config:        mockProviderConfig(unknown) + fmt.Sprintf(scopedKubeconfigConfig, ""),
				ResourceName:  "portainer_kubernetes_scoped_kubeconfig.k",
				ImportState:   true,
				ImportStateId: "2:5",
				ExpectError:   regexp.MustCompile(`scoped kubeconfig 5 not found on environment 2`),
			},
			{
				Config:        mockProviderConfig(unknown) + fmt.Sprintf(scopedKubeconfigConfig, ""),
				ResourceName:  "portainer_kubernetes_scoped_kubeconfig.k",
				ImportState:   true,
				ImportStateId: "2:x",
				ExpectError:   regexp.MustCompile(`expected <environment_id>:<scoped_kubeconfig_id>`),
			},
		},
	})
}

func TestKubernetesScopedKubeconfig_UpgradeFromV300(t *testing.T) {
	mock := NewMockServer(t)
	newFakeScopedKubeconfigs(mock)
	a2Upgrade(t, mock,
		fmt.Sprintf(scopedKubeconfigConfig, ""),
		fmt.Sprintf(scopedKubeconfigConfig, `
  namespaces      = ["team-a"]
  include_secrets = true
  description     = "ci"
`),
	)
}

// TestKubernetesScopedKubeconfig_ImportWithoutNamespaces: Portainer reports
// the covered namespaces, not the issued ones, so after importing a
// kubeconfig a configuration leaving namespaces unset, or listing them in
// another order, updates the state in place (with a warning when unset)
// instead of issuing a new credential. Any other change still replaces it.
func TestKubernetesScopedKubeconfig_ImportWithoutNamespaces(t *testing.T) {
	for name, tc := range map[string]struct {
		namespaces string
		action     plancheck.ResourceActionType
	}{
		"unset":      {"", plancheck.ResourceActionUpdate},
		"reordered":  {`namespaces = ["team-a", "default"]`, plancheck.ResourceActionUpdate},
		"same order": {`namespaces = ["default", "team-a"]`, plancheck.ResourceActionNoop},
		"other":      {`namespaces = ["team-b"]`, plancheck.ResourceActionReplace},
	} {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			fake := newFakeScopedKubeconfigs(mock)
			fake.store.put("5", map[string]interface{}{
				"id": 5, "endpointId": 2, "description": "", "namespaces": []string{"default", "team-a"},
				"includeSecrets": false, "serviceAccount": "portainer-sa-5", "active": true,
				"createdAt": 1000, "expiresAt": 4600,
			})
			config := mockProviderConfig(mock) + fmt.Sprintf(scopedKubeconfigConfig, tc.namespaces)
			steps := []resource.TestStep{
				{
					Config:             config,
					ResourceName:       "portainer_kubernetes_scoped_kubeconfig.k",
					ImportState:        true,
					ImportStateId:      "2:5",
					ImportStatePersist: true,
					ImportStateCheck: func(states []*terraform.InstanceState) error {
						if got := states[0].Attributes["namespaces.#"]; got != "2" {
							return fmt.Errorf("namespaces.# = %q, want the 2 covered ones", got)
						}
						return nil
					},
				},
				{
					Config: config,
					ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("portainer_kubernetes_scoped_kubeconfig.k", tc.action),
					}},
					Check: a2Check(func() error {
						issued := a2Count(mock, "POST", "/kubernetes/2/scoped_kubeconfig")
						if replaced := tc.action == plancheck.ResourceActionReplace; replaced != (issued == 1) {
							return fmt.Errorf("%d kubeconfigs issued for a %s", issued, tc.action)
						}
						return nil
					}),
				},
			}
			if tc.action != plancheck.ResourceActionReplace {
				steps = append(steps, resource.TestStep{Config: config, PlanOnly: true})
			}
			resource.UnitTest(t, resource.TestCase{ProtoV5ProviderFactories: testProtoV5Factories, Steps: steps})
		})
	}
}

// TestKubernetesScopedKubeconfig_ImportMarkCleared: the import leniency lasts
// for the first plan after the import only. When that plan has nothing to
// change (the configuration matches exactly), leaving namespaces unset later
// issues a new kubeconfig instead of only updating the state.
func TestKubernetesScopedKubeconfig_ImportMarkCleared(t *testing.T) {
	mock := NewMockServer(t)
	fake := newFakeScopedKubeconfigs(mock)
	fake.store.put("5", map[string]interface{}{
		"id": 5, "endpointId": 2, "description": "", "namespaces": []string{"default", "team-a"},
		"includeSecrets": false, "serviceAccount": "portainer-sa-5", "active": true,
		"createdAt": 1000, "expiresAt": 4600,
	})
	matching := mockProviderConfig(mock) + fmt.Sprintf(scopedKubeconfigConfig, `namespaces = ["default", "team-a"]`)
	unset := mockProviderConfig(mock) + fmt.Sprintf(scopedKubeconfigConfig, "")
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config:             matching,
				ResourceName:       "portainer_kubernetes_scoped_kubeconfig.k",
				ImportState:        true,
				ImportStateId:      "2:5",
				ImportStatePersist: true,
			},
			{
				Config: matching,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("portainer_kubernetes_scoped_kubeconfig.k", plancheck.ResourceActionNoop),
				}},
			},
			{
				Config: unset,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("portainer_kubernetes_scoped_kubeconfig.k", plancheck.ResourceActionReplace),
				}},
				Check: a2Check(func() error {
					if issued := a2Count(mock, "POST", "/kubernetes/2/scoped_kubeconfig"); issued != 1 {
						return fmt.Errorf("%d kubeconfigs issued, want 1", issued)
					}
					return nil
				}),
			},
		},
	})
}
