package internal

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// fakeGatewayAPI is a stateful stand-in for Portainer's Gateway API
// endpoints of environment 1, namespace "web": kind is "gateways" or
// "gatewayhttproutes". defaults fills what Kubernetes would.
type fakeGatewayAPI struct {
	mu       sync.Mutex
	kind     string
	objects  []map[string]interface{}
	version  int
	defaults func(map[string]interface{})
	// stale makes every PUT fail as if another client had changed the
	// object between the provider's read and its update.
	stale bool
}

func newFakeGatewayAPI(mock *MockServer, kind string, defaults func(map[string]interface{})) *fakeGatewayAPI {
	f := &fakeGatewayAPI{kind: kind, defaults: defaults}
	base := "/kubernetes/1/namespaces/web/" + kind
	mock.On("GET", base, func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		RespondJSON(http.StatusOK, append([]map[string]interface{}{}, f.objects...))(w, r)
	})
	mock.On("POST", base, func(w http.ResponseWriter, r *http.Request) {
		var obj map[string]interface{}
		a2Decode(r, &obj)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.store(obj, false)
		w.WriteHeader(http.StatusNoContent)
	})
	mock.On("PUT", base, func(w http.ResponseWriter, r *http.Request) {
		var obj map[string]interface{}
		a2Decode(r, &obj)
		f.mu.Lock()
		defer f.mu.Unlock()
		for _, o := range f.objects {
			if o["Name"] == obj["Name"] && (f.stale || o["ResourceVersion"] != obj["ResourceVersion"]) {
				RespondString(http.StatusConflict, "", "stale resource version")(w, r)
				return
			}
		}
		f.store(obj, true)
		w.WriteHeader(http.StatusNoContent)
	})
	mock.On("POST", "/kubernetes/1/"+kind+"/delete", func(w http.ResponseWriter, r *http.Request) {
		var body map[string][]string
		a2Decode(r, &body)
		f.mu.Lock()
		defer f.mu.Unlock()
		for _, name := range body["web"] {
			f.remove(name)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	return f
}

func (f *fakeGatewayAPI) remove(name string) {
	kept := f.objects[:0]
	for _, o := range f.objects {
		if o["Name"] != name {
			kept = append(kept, o)
		}
	}
	f.objects = kept
}

func (f *fakeGatewayAPI) store(obj map[string]interface{}, update bool) {
	f.version++
	obj["ResourceVersion"] = strconv.Itoa(f.version)
	if f.defaults != nil {
		f.defaults(obj)
	}
	if update {
		for i, o := range f.objects {
			if o["Name"] == obj["Name"] {
				for _, k := range []string{"UID", "CreationDate"} {
					if v, ok := o[k]; ok {
						obj[k] = v
					}
				}
				f.objects[i] = obj
				return
			}
		}
	}
	f.objects = append(f.objects, obj)
}

// edit changes the stored object outside Terraform.
func (f *fakeGatewayAPI) edit(name string, change func(map[string]interface{})) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, o := range f.objects {
		if o["Name"] == name {
			change(o)
			f.version++
			o["ResourceVersion"] = strconv.Itoa(f.version)
		}
	}
}

func (f *fakeGatewayAPI) clear() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects = nil
}

// gatewayDefaults is what Portainer adds to a Gateway it stores.
func gatewayDefaults(obj map[string]interface{}) {
	labels, _ := obj["Labels"].(map[string]interface{})
	if labels == nil {
		labels = map[string]interface{}{}
	}
	labels["io.portainer.kubernetes.gateway.owner"] = "admin"
	obj["Labels"] = labels
	if _, ok := obj["UID"]; !ok {
		obj["UID"] = "uid-1"
		obj["CreationDate"] = "2026-11-16T00:00:00Z"
	}
}

const gatewayConfig = `
resource "portainer_kubernetes_gateway" "g" {
  environment_id = 1
  namespace      = "web"
  name           = "edge"
  class_name     = %q
  listener {
    name     = "http"
    port     = 80
    protocol = "HTTP"
  }
  listener {
    name     = "https"
    port     = 443
    protocol = "HTTPS"
    hostname = "*.example.com"
  }
  %s
}
`

// TestKubernetesGateway_Lifecycle: the flat model is posted, Portainer's own
// labels stay out of state, an update sends the resource version last read,
// drift and deletion are detected, import restores everything, and destroy
// goes through the bulk delete.
func TestKubernetesGateway_Lifecycle(t *testing.T) {
	mock := NewMockServer(t)
	fake := newFakeGatewayAPI(mock, "gateways", gatewayDefaults)
	extras := `
  address {
    type  = "IPAddress"
    value = "10.0.0.1"
  }
  labels      = { team = "web" }
  annotations = { note = "x" }
`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(gatewayConfig, "cilium", extras),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_kubernetes_gateway.g", "id", "1:web:edge"),
					resource.TestCheckResourceAttr("portainer_kubernetes_gateway.g", "uid", "uid-1"),
					resource.TestCheckResourceAttr("portainer_kubernetes_gateway.g", "resource_version", "1"),
					resource.TestCheckResourceAttr("portainer_kubernetes_gateway.g", "labels.%", "1"),
					resource.TestCheckNoResourceAttr("portainer_kubernetes_gateway.g", "listener.0.hostname"),
					a2Check(func() error {
						var body map[string]interface{}
						a2LastBody(t, mock, "POST", "/kubernetes/1/namespaces/web/gateways", &body)
						if body["ClassName"] != "cilium" || body["ResourceVersion"] != nil {
							return fmt.Errorf("create payload: %v", body)
						}
						listeners := body["Listeners"].([]interface{})
						if len(listeners) != 2 || listeners[1].(map[string]interface{})["Hostname"] != "*.example.com" {
							return fmt.Errorf("listeners payload: %v", listeners)
						}
						if _, ok := listeners[0].(map[string]interface{})["Hostname"]; ok {
							return fmt.Errorf("an unset hostname must be left out: %v", listeners[0])
						}
						return nil
					}),
				),
			},
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(gatewayConfig, "envoy", extras),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("portainer_kubernetes_gateway.g", plancheck.ResourceActionUpdate),
				}},
				Check: a2Check(func() error {
					var body map[string]interface{}
					a2LastBody(t, mock, "PUT", "/kubernetes/1/namespaces/web/gateways", &body)
					if body["ResourceVersion"] != "1" || body["ClassName"] != "envoy" {
						return fmt.Errorf("update payload: %v", body)
					}
					return nil
				}),
			},
			{
				PreConfig: func() {
					fake.edit("edge", func(o map[string]interface{}) { o["ClassName"] = "other" })
				},
				Config:             mockProviderConfig(mock) + fmt.Sprintf(gatewayConfig, "envoy", extras),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				// Apply the drift back.
				Config: mockProviderConfig(mock) + fmt.Sprintf(gatewayConfig, "envoy", extras),
			},
			{
				ResourceName:      "portainer_kubernetes_gateway.g",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				PreConfig:          fake.clear,
				Config:             mockProviderConfig(mock) + fmt.Sprintf(gatewayConfig, "envoy", extras),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestKubernetesGateway_Destroy(t *testing.T) {
	mock := NewMockServer(t)
	newFakeGatewayAPI(mock, "gateways", gatewayDefaults)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: mockProviderConfig(mock) + fmt.Sprintf(gatewayConfig, "cilium", "")},
		},
		CheckDestroy: a2Check(func() error {
			var body map[string][]string
			a2LastBody(t, mock, "POST", "/kubernetes/1/gateways/delete", &body)
			if names := body["web"]; len(names) != 1 || names[0] != "edge" {
				return fmt.Errorf("delete payload: %v", body)
			}
			return nil
		}),
	})
}

func TestKubernetesGateway_Errors(t *testing.T) {
	old := NewMockServer(t)
	old.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{"ServerVersion": "2.45.3", "ServerEdition": "BE"}))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config:      mockProviderConfig(old) + fmt.Sprintf(gatewayConfig, "cilium", ""),
				ExpectError: regexp.MustCompile(`portainer_kubernetes_gateway\s+requires`),
			},
			{
				Config:      mockProviderConfig(old) + `resource "portainer_kubernetes_gateway" "g" {` + "\n" + `environment_id = 1` + "\n" + `namespace = "web"` + "\n" + `name = "edge"` + "\n" + `class_name = "x"` + "\n}",
				ExpectError: regexp.MustCompile(`(?s)listener.*(at least 1|must have a configuration value)`),
			},
		},
	})

	mock := NewMockServer(t)
	fake := newFakeGatewayAPI(mock, "gateways", gatewayDefaults)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: mockProviderConfig(mock) + fmt.Sprintf(gatewayConfig, "cilium", "")},
			{
				// A concurrent change makes the version sent stale.
				PreConfig: func() {
					fake.mu.Lock()
					fake.stale = true
					fake.mu.Unlock()
				},
				Config:      mockProviderConfig(mock) + fmt.Sprintf(gatewayConfig, "envoy", ""),
				ExpectError: regexp.MustCompile(`failed to update gateway web/edge`),
			},
		},
	})
}

func TestKubernetesGateway_UpgradeFromV300(t *testing.T) {
	mock := NewMockServer(t)
	newFakeGatewayAPI(mock, "gateways", gatewayDefaults)
	a2Upgrade(t, mock,
		fmt.Sprintf(gatewayConfig, "cilium", ""),
		fmt.Sprintf(gatewayConfig, "cilium", `
  address {
    value = "10.0.0.1"
  }
  labels = { team = "web" }
`),
	)
}

// TestKubernetesGateway_ForeignMetadata: annotations and labels someone else
// adds stay out of state (no drift), an update sends them back unchanged, a
// key removed from the configuration is deleted, a managed key changed in
// the cluster is drift, and an import takes every key but Portainer's own.
func TestKubernetesGateway_ForeignMetadata(t *testing.T) {
	mock := NewMockServer(t)
	fake := newFakeGatewayAPI(mock, "gateways", gatewayDefaults)
	config := func(extras string) string {
		return mockProviderConfig(mock) + fmt.Sprintf(gatewayConfig, "cilium", extras)
	}
	managed := `
  labels      = { team = "web", tier = "edge" }
  annotations = { note = "x" }
`
	addForeign := func() {
		fake.edit("edge", func(o map[string]interface{}) {
			o["Annotations"].(map[string]interface{})["controller.example.com/status"] = "ready"
			o["Labels"].(map[string]interface{})["app.kubernetes.io/managed-by"] = "kubectl"
		})
	}
	sent := func(key string) map[string]interface{} {
		var body map[string]interface{}
		a2LastBody(t, mock, "PUT", "/kubernetes/1/namespaces/web/gateways", &body)
		m, _ := body[key].(map[string]interface{})
		return m
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config(managed)},
			{
				PreConfig: addForeign,
				Config:    config(managed),
				PlanOnly:  true,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_kubernetes_gateway.g", "annotations.%", "1"),
					resource.TestCheckResourceAttr("portainer_kubernetes_gateway.g", "labels.%", "2"),
				),
			},
			{
				// tier is removed from the configuration, note changes.
				Config: config(`
  labels      = { team = "web" }
  annotations = { note = "y" }
`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("portainer_kubernetes_gateway.g", plancheck.ResourceActionUpdate),
				}},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_kubernetes_gateway.g", "annotations.%", "1"),
					resource.TestCheckResourceAttr("portainer_kubernetes_gateway.g", "labels.%", "1"),
					a2Check(func() error {
						ann, labels := sent("Annotations"), sent("Labels")
						if len(ann) != 2 || ann["note"] != "y" || ann["controller.example.com/status"] != "ready" {
							return fmt.Errorf("annotations sent: %v", ann)
						}
						if _, ok := labels["tier"]; ok || labels["team"] != "web" || labels["app.kubernetes.io/managed-by"] != "kubectl" || labels["io.portainer.kubernetes.gateway.owner"] != "admin" {
							return fmt.Errorf("labels sent: %v", labels)
						}
						return nil
					}),
				),
			},
			{
				// A managed key changed or removed in the cluster is drift.
				PreConfig: func() {
					fake.edit("edge", func(o map[string]interface{}) {
						o["Annotations"].(map[string]interface{})["note"] = "changed"
						delete(o["Labels"].(map[string]interface{}), "team")
					})
				},
				Config: config(`
  labels      = { team = "web" }
  annotations = { note = "y" }
`),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: config(`
  labels      = { team = "web" }
  annotations = { note = "y" }
`),
				Check: a2Check(func() error {
					if ann := sent("Annotations"); ann["note"] != "y" || ann["controller.example.com/status"] != "ready" {
						return fmt.Errorf("annotations sent: %v", ann)
					}
					return nil
				}),
			},
			{
				// No annotations or labels configured: the foreign ones
				// still do not show.
				Config: config(""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("portainer_kubernetes_gateway.g", "annotations.%"),
					resource.TestCheckNoResourceAttr("portainer_kubernetes_gateway.g", "labels.%"),
					a2Check(func() error {
						if ann := sent("Annotations"); len(ann) != 1 || ann["controller.example.com/status"] != "ready" {
							return fmt.Errorf("annotations sent: %v", ann)
						}
						return nil
					}),
				),
			},
			{
				Config:             config(""),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{
				// An import takes every key but Portainer's own.
				ResourceName:            "portainer_kubernetes_gateway.g",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"annotations", "labels"},
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					attrs := states[0].Attributes
					if attrs["annotations.controller.example.com/status"] != "ready" || attrs["labels.app.kubernetes.io/managed-by"] != "kubectl" {
						return fmt.Errorf("imported: %v", attrs)
					}
					if _, ok := attrs["labels.io.portainer.kubernetes.gateway.owner"]; ok {
						return fmt.Errorf("Portainer's own label imported: %v", attrs)
					}
					return nil
				},
			},
		},
	})
}
