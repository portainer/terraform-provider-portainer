package internal

import (
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// fakeNamespaces is a stateful stand-in for Portainer's namespace API of
// environment 1. Quantities come back canonical ("1000m" as "1").
type fakeNamespaces struct {
	mu       sync.Mutex
	licensed bool
	ns       map[string]map[string]interface{}
}

func canonicalQuantity(v string) string {
	if v == "1000m" {
		return "1"
	}
	return v
}

func newFakeNamespaces(mock *MockServer, licensed bool) *fakeNamespaces {
	f := &fakeNamespaces{licensed: licensed, ns: map[string]map[string]interface{}{}}
	mock.On("GET", "/licenses", func(w http.ResponseWriter, r *http.Request) {
		if f.licensed {
			RespondJSON(http.StatusOK, []map[string]interface{}{{"id": 1}})(w, r)
			return
		}
		RespondJSON(http.StatusOK, []map[string]interface{}{})(w, r)
	})
	mock.On("POST", "/kubernetes/1/namespaces", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		a2Decode(r, &body)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.store(body)
		RespondJSON(http.StatusOK, map[string]interface{}{})(w, r)
	})
	mock.On("DELETE", "/kubernetes/1/namespaces", func(w http.ResponseWriter, r *http.Request) {
		var names []string
		a2Decode(r, &names)
		f.mu.Lock()
		defer f.mu.Unlock()
		for _, n := range names {
			delete(f.ns, n)
		}
		RespondJSON(http.StatusOK, map[string]interface{}{})(w, r)
	})
	return f
}

// serve registers GET and PUT of a namespace name the test uses.
func (f *fakeNamespaces) serve(mock *MockServer, names ...string) {
	for _, name := range names {
		name := name
		mock.On("GET", "/kubernetes/1/namespaces/"+name, func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			if ns, ok := f.ns[name]; ok {
				RespondJSON(http.StatusOK, ns)(w, r)
				return
			}
			RespondString(http.StatusNotFound, "application/json", `{"message":"not found"}`)(w, r)
		})
		mock.On("PUT", "/kubernetes/1/namespaces/"+name, func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			a2Decode(r, &body)
			f.mu.Lock()
			defer f.mu.Unlock()
			delete(f.ns, name)
			f.store(body)
			RespondJSON(http.StatusOK, map[string]interface{}{})(w, r)
		})
	}
}

func (f *fakeNamespaces) store(body map[string]interface{}) {
	name, _ := body["Name"].(string)
	ns := map[string]interface{}{"Name": name}
	rq, _ := body["ResourceQuota"].(map[string]interface{})
	hard := map[string]string{}
	if rq["enabled"] == true {
		str := func(k string) string { s, _ := rq[k].(string); return canonicalQuantity(s) }
		if f.licensed {
			hard["limits.cpu"], hard["requests.cpu"] = str("cpuLimit"), str("cpuRequest")
			hard["limits.memory"], hard["requests.memory"] = str("memoryLimit"), str("memoryRequest")
		} else {
			hard["limits.cpu"], hard["requests.cpu"] = str("cpu"), str("cpu")
			hard["limits.memory"], hard["requests.memory"] = str("memory"), str("memory")
		}
		ns["ResourceQuota"] = map[string]interface{}{"spec": map[string]interface{}{"hard": hard}}
		ns["ResourceQuotas"] = []map[string]interface{}{{
			"metadata": map[string]interface{}{"name": "portainer-rq-" + name},
			"spec":     map[string]interface{}{"hard": hard},
			"status":   map[string]interface{}{"hard": hard, "used": map[string]string{"limits.cpu": "100m"}},
		}}
		ns["EffectiveCeiling"] = map[string]interface{}{
			"limits.memory": map[string]string{"BoundBy": "portainer-rq-" + name, "Hard": hard["limits.memory"], "Used": "0"},
			"limits.cpu":    map[string]string{"BoundBy": "portainer-rq-" + name, "Hard": hard["limits.cpu"], "Used": "100m"},
		}
	}
	// The Business Edition quotas land in the same spec.hard.
	if lb, _ := body["LoadBalancerQuota"].(map[string]interface{}); lb["enabled"] == true {
		hard[namespaceLoadBalancerHardKey] = fmt.Sprint(lb["limit"])
	}
	storage, _ := body["StorageQuotas"].(map[string]interface{})
	for class, q := range storage {
		if q, _ := q.(map[string]interface{}); q["enabled"] == true {
			limit, _ := q["limit"].(string)
			if limit == "10240Mi" {
				limit = "10Gi"
			}
			hard[fmt.Sprintf(namespaceStorageClassHardKeyFn, class)] = limit
		}
	}
	if len(hard) > 0 && ns["ResourceQuota"] == nil {
		ns["ResourceQuota"] = map[string]interface{}{"spec": map[string]interface{}{"hard": hard}}
	}
	f.ns[name] = ns
}

const namespaceConfig = `
resource "portainer_kubernetes_namespace" "n" {
  environment_id = 1
  name           = %q
  %s
}
`

// TestKubernetesNamespace_LifecycleUnlicensed: without a license the quota
// is one cpu/memory pair, the owner and annotations go out, Portainer's
// canonical quantities plan no change, the quotas and ceilings are reported
// sorted, a rename puts to the old name, import works and destroy sends a
// list of names.
func TestKubernetesNamespace_LifecycleUnlicensed(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeNamespaces(mock, false)
	fake.serve(mock, "my-ns", "new-ns")
	body := `
  owner       = "admin"
  annotations = { team = "platform" }
  resource_quota = {
    cpu    = "1000m"
    memory = "512Mi"
  }
`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(namespaceConfig, "my-ns", body),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_kubernetes_namespace.n", "id", "1:my-ns"),
					resource.TestCheckResourceAttr("portainer_kubernetes_namespace.n", "resource_quota.cpu", "1000m"),
					resource.TestCheckResourceAttr("portainer_kubernetes_namespace.n", "resource_quotas.#", "1"),
					resource.TestCheckResourceAttr("portainer_kubernetes_namespace.n", "resource_quotas.0.name", "portainer-rq-my-ns"),
					resource.TestCheckResourceAttr("portainer_kubernetes_namespace.n", "resource_quotas.0.hard.limits.memory", "512Mi"),
					resource.TestCheckResourceAttr("portainer_kubernetes_namespace.n", "effective_ceiling.0.resource", "limits.cpu"),
					resource.TestCheckResourceAttr("portainer_kubernetes_namespace.n", "effective_ceiling.1.resource", "limits.memory"),
					a2Check(func() error {
						var payload map[string]interface{}
						a2LastBody(t, mock, "POST", "/kubernetes/1/namespaces", &payload)
						rq := payload["ResourceQuota"].(map[string]interface{})
						if payload["Name"] != "my-ns" || payload["Owner"] != "admin" || rq["enabled"] != true || rq["cpu"] != "1000m" || rq["memory"] != "512Mi" {
							return fmt.Errorf("create payload: %v", payload)
						}
						if ann := payload["Annotations"].(map[string]interface{}); ann["team"] != "platform" {
							return fmt.Errorf("annotations: %v", payload["Annotations"])
						}
						if r := mock.FindRequest("GET", "/kubernetes/1/namespaces/my-ns"); r == nil || r.Query != "withResourceQuota=true" {
							return fmt.Errorf("expected withResourceQuota=true, got %+v", r)
						}
						return nil
					}),
				),
			},
			{
				// "1000m" comes back as "1": the same quota.
				Config:   mockProviderConfig(mock) + fmt.Sprintf(namespaceConfig, "my-ns", body),
				PlanOnly: true,
			},
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(namespaceConfig, "new-ns", body),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("portainer_kubernetes_namespace.n", plancheck.ResourceActionUpdate),
				}},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_kubernetes_namespace.n", "id", "1:new-ns"),
					a2Check(func() error {
						if a2Count(mock, "PUT", "/kubernetes/1/namespaces/my-ns") != 1 {
							return fmt.Errorf("a rename must PUT to the old name")
						}
						return nil
					}),
				),
			},
			{
				ResourceName:            "portainer_kubernetes_namespace.n",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"annotations", "owner", "resource_quota"},
			},
		},
		CheckDestroy: a2Check(func() error {
			var names []string
			a2LastBody(t, mock, "DELETE", "/kubernetes/1/namespaces", &names)
			if !reflect.DeepEqual(names, []string{"new-ns"}) {
				return fmt.Errorf("delete payload: %v", names)
			}
			return nil
		}),
	})
}

// TestKubernetesNamespace_Licensed: with a license the four limits and
// requests are sent and read back under their own keys; without
// resource_quota the quota is disabled.
func TestKubernetesNamespace_Licensed(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeNamespaces(mock, true)
	fake.serve(mock, "team-a")
	quota := `
  resource_quota = {
    cpu_request    = "500m"
    cpu_limit      = "2"
    memory_request = "256Mi"
    memory_limit   = "1Gi"
  }
`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(namespaceConfig, "team-a", quota),
				Check: a2Check(func() error {
					var payload map[string]interface{}
					a2LastBody(t, mock, "POST", "/kubernetes/1/namespaces", &payload)
					rq := payload["ResourceQuota"].(map[string]interface{})
					want := map[string]interface{}{"enabled": true, "cpuRequest": "500m", "cpuLimit": "2", "memoryRequest": "256Mi", "memoryLimit": "1Gi"}
					if !reflect.DeepEqual(rq, want) {
						return fmt.Errorf("ResourceQuota: %v", rq)
					}
					return nil
				}),
			},
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(namespaceConfig, "team-a", ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("portainer_kubernetes_namespace.n", "resource_quota"),
					a2Check(func() error {
						var payload map[string]interface{}
						a2LastBody(t, mock, "PUT", "/kubernetes/1/namespaces/team-a", &payload)
						if rq := payload["ResourceQuota"].(map[string]interface{}); !reflect.DeepEqual(rq, map[string]interface{}{"enabled": false}) {
							return fmt.Errorf("expected a disabled quota, got %v", rq)
						}
						return nil
					}),
				),
			},
		},
	})
}

func TestKubernetesNamespace_ErrorsAndDrift(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeNamespaces(mock, false)
	fake.serve(mock, "my-ns")
	config := mockProviderConfig(mock) + fmt.Sprintf(namespaceConfig, "my-ns", "")
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					fake.mu.Lock()
					delete(fake.ns, "my-ns")
					fake.mu.Unlock()
				},
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})

	failing := NewMockServer(t)
	failing.On("GET", "/licenses", RespondString(http.StatusOK, "application/json", "this is not json"))
	conflict := NewMockServer(t)
	conflict.On("GET", "/licenses", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	conflict.On("POST", "/kubernetes/1/namespaces", RespondString(http.StatusConflict, "application/json", `{"message":"namespace exists"}`))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config:      mockProviderConfig(failing) + fmt.Sprintf(namespaceConfig, "my-ns", ""),
				ExpectError: regexp.MustCompile(`invalid character`),
			},
			{
				Config:      mockProviderConfig(conflict) + fmt.Sprintf(namespaceConfig, "my-ns", ""),
				ExpectError: regexp.MustCompile(`failed to create namespace`),
			},
			{
				Config:      mockProviderConfig(conflict) + fmt.Sprintf(namespaceConfig, "", ""),
				ExpectError: regexp.MustCompile(`at least 1`),
			},
			{
				Config:        mockProviderConfig(conflict) + fmt.Sprintf(namespaceConfig, "my-ns", ""),
				ResourceName:  "portainer_kubernetes_namespace.n",
				ImportState:   true,
				ImportStateId: "nocolon",
				ExpectError:   regexp.MustCompile(`invalid ID format, expected 'envID:name'`),
			},
		},
	})
}

func TestKubernetesNamespace_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeNamespaces(mock, false)
	fake.serve(mock, "plain", "quota")
	a2Upgrade(t, mock,
		fmt.Sprintf(namespaceConfig, "plain", ""),
		fmt.Sprintf(namespaceConfig, "quota", `
  owner          = "admin"
  annotations    = { team = "platform" }
  resource_quota = { cpu = "2", memory = "512Mi" }
`),
	)
}

// TestKubernetesNamespace_UpgradeNonCanonicalQuantity documents the one
// difference from v3.0.0: a quota written in a non-canonical form ("1000m",
// which Kubernetes reports as "1") was suppressed in the plan by SDKv2 and
// is planned as one in-place update now (the framework cannot suppress a
// difference between configuration and state). The update sends the same
// quota, and the plans after it are empty.
func TestKubernetesNamespace_UpgradeNonCanonicalQuantity(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeNamespaces(mock, false)
	fake.serve(mock, "my-ns")
	config := mockProviderConfig(mock) + fmt.Sprintf(namespaceConfig, "my-ns", `resource_quota = { cpu = "1000m", memory = "512Mi" }`)
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		Steps: []resource.TestStep{
			{ExternalProviders: previousRelease, Config: config},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   config,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("portainer_kubernetes_namespace.n", plancheck.ResourceActionUpdate),
				}},
			},
			{ProtoV5ProviderFactories: testProtoV5Factories, Config: config, PlanOnly: true},
		},
	})
}

func TestNamespaceQuotaKeepAuthored(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		old, live, want string
	}{
		{"1000m", "1", "1000m"},
		{"1024Mi", "1Gi", "1024Mi"},
		{"500m", "1", "1"},
		{"", "1", "1"},
	} {
		got := namespaceQuotaKeepAuthored(map[string]string{"cpu": tc.live}, map[string]interface{}{"cpu": tc.old})
		if got["cpu"] != tc.want {
			t.Errorf("%q in state, %q live: got %q, want %q", tc.old, tc.live, got["cpu"], tc.want)
		}
	}
}

// TestNamespaceQuotaFromHard: the keys follow the configuration; an import
// uses cpu/memory when the limits equal the requests.
func TestNamespaceQuotaFromHard(t *testing.T) {
	t.Parallel()
	hard := map[string]string{"limits.cpu": "2", "requests.cpu": "2", "limits.memory": "1Gi", "requests.memory": "1Gi"}
	for _, tc := range []struct {
		name       string
		configured map[string]interface{}
		want       map[string]string
	}{
		{"combined", map[string]interface{}{"cpu": "2"}, map[string]string{"cpu": "2", "memory": "1Gi"}},
		{"split", map[string]interface{}{"cpu_limit": "2"}, map[string]string{"cpu_limit": "2", "cpu_request": "2", "memory_limit": "1Gi", "memory_request": "1Gi"}},
		{"import", nil, map[string]string{"cpu": "2", "memory": "1Gi"}},
	} {
		if got := namespaceQuotaFromHard(hard, tc.configured); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
	split := map[string]string{"limits.cpu": "2", "requests.cpu": "500m"}
	if got := namespaceQuotaFromHard(split, nil); !strings.Contains(fmt.Sprint(got), "cpu_request") {
		t.Errorf("import with different requests: got %v", got)
	}
}

// TestKubernetesNamespace_BusinessEditionQuotas: load_balancer_quota and
// storage_quotas reach Portainer, an equal quantity in another form
// ("10240Mi" reported as "10Gi") plans no change, drift shows, removing them
// disables the quotas, and once unmanaged they are no longer sent.
func TestKubernetesNamespace_BusinessEditionQuotas(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeNamespaces(mock, true)
	fake.serve(mock, "team-b")
	quotas := `
  storage_quotas = { standard = "10240Mi", fast = "1Gi" }
  load_balancer_quota {
    limit = 3
  }
`
	lastPut := func() map[string]interface{} {
		var payload map[string]interface{}
		a2LastBody(t, mock, "PUT", "/kubernetes/1/namespaces/team-b", &payload)
		return payload
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(namespaceConfig, "team-b", quotas),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_kubernetes_namespace.n", "storage_quotas.standard", "10240Mi"),
					resource.TestCheckResourceAttr("portainer_kubernetes_namespace.n", "load_balancer_quota.0.enabled", "true"),
					a2Check(func() error {
						var payload map[string]interface{}
						a2LastBody(t, mock, "POST", "/kubernetes/1/namespaces", &payload)
						if lb := payload["LoadBalancerQuota"]; !reflect.DeepEqual(lb, map[string]interface{}{"enabled": true, "limit": float64(3)}) {
							return fmt.Errorf("LoadBalancerQuota: %v", lb)
						}
						want := map[string]interface{}{
							"standard": map[string]interface{}{"enabled": true, "limit": "10240Mi"},
							"fast":     map[string]interface{}{"enabled": true, "limit": "1Gi"},
						}
						if sq := payload["StorageQuotas"]; !reflect.DeepEqual(sq, want) {
							return fmt.Errorf("StorageQuotas: %v", sq)
						}
						return nil
					}),
				),
			},
			{
				PreConfig: func() {
					fake.mu.Lock()
					hard := fake.ns["team-b"]["ResourceQuota"].(map[string]interface{})["spec"].(map[string]interface{})["hard"].(map[string]string)
					hard[namespaceLoadBalancerHardKey] = "5"
					fake.mu.Unlock()
				},
				Config:             mockProviderConfig(mock) + fmt.Sprintf(namespaceConfig, "team-b", quotas),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(namespaceConfig, "team-b", `storage_quotas = { standard = "10Gi" }`),
				Check: a2Check(func() error {
					payload := lastPut()
					if lb := payload["LoadBalancerQuota"]; !reflect.DeepEqual(lb, map[string]interface{}{"enabled": false}) {
						return fmt.Errorf("expected a disabled LoadBalancerQuota, got %v", lb)
					}
					want := map[string]interface{}{
						"standard": map[string]interface{}{"enabled": true, "limit": "10Gi"},
						"fast":     map[string]interface{}{"enabled": false},
					}
					if sq := payload["StorageQuotas"]; !reflect.DeepEqual(sq, want) {
						return fmt.Errorf("StorageQuotas: %v", sq)
					}
					return nil
				}),
			},
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(namespaceConfig, "team-b", ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("portainer_kubernetes_namespace.n", "storage_quotas"),
					a2Check(func() error {
						want := map[string]interface{}{"standard": map[string]interface{}{"enabled": false}}
						if sq := lastPut()["StorageQuotas"]; !reflect.DeepEqual(sq, want) {
							return fmt.Errorf("StorageQuotas: %v", sq)
						}
						return nil
					}),
				),
			},
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(namespaceConfig, "team-b", `owner = "admin"`),
				Check: a2Check(func() error {
					payload := lastPut()
					for _, key := range []string{"LoadBalancerQuota", "StorageQuotas"} {
						if _, ok := payload[key]; ok {
							return fmt.Errorf("%s: expected it left out when not managed", key)
						}
					}
					return nil
				}),
			},
		},
	})
}

// TestKubernetesNamespace_BusinessEditionQuotas_UpgradeFromV300: a namespace
// with LoadBalancer and storage quotas set outside Terraform plans no change
// after upgrading.
func TestKubernetesNamespace_BusinessEditionQuotas_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeNamespaces(mock, true)
	fake.serve(mock, "team-c")
	config := mockProviderConfig(mock) + fmt.Sprintf(namespaceConfig, "team-c", `resource_quota = { cpu_limit = "2", cpu_request = "1", memory_limit = "1Gi", memory_request = "512Mi" }`)
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		Steps: []resource.TestStep{
			{ExternalProviders: previousRelease, Config: config},
			{
				PreConfig: func() {
					fake.mu.Lock()
					hard := fake.ns["team-c"]["ResourceQuota"].(map[string]interface{})["spec"].(map[string]interface{})["hard"].(map[string]string)
					hard[namespaceLoadBalancerHardKey] = "2"
					hard[fmt.Sprintf(namespaceStorageClassHardKeyFn, "standard")] = "5Gi"
					fake.mu.Unlock()
				},
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}
