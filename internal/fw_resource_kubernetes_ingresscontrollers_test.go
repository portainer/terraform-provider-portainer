package internal

import (
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// fakeIngressControllers serves the controllers of a cluster (or namespace)
// at path. Portainer fills name, type, used and new itself and takes only
// the availability per class on update.
type fakeIngressControllers struct {
	mu          sync.Mutex
	controllers []IngressController
}

func newFakeIngressControllers(mock *MockServer, path string) *fakeIngressControllers {
	f := &fakeIngressControllers{controllers: []IngressController{
		{Name: "nginx", ClassName: "nginx", Type: "nginx", Availability: true, Used: true},
		{Name: "traefik", ClassName: "traefik", Type: "traefik", Availability: true, New: true},
	}}
	mock.On("GET", path, func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		RespondJSON(http.StatusOK, f.controllers)(w, r)
	})
	mock.On("PUT", path, func(w http.ResponseWriter, r *http.Request) {
		var body []IngressController
		a2Decode(r, &body)
		f.mu.Lock()
		defer f.mu.Unlock()
		for _, want := range body {
			for i := range f.controllers {
				if f.controllers[i].ClassName == want.ClassName {
					f.controllers[i].Availability = want.Availability
				}
			}
		}
		w.WriteHeader(http.StatusNoContent)
	})
	return f
}

func (f *fakeIngressControllers) setAvailability(class string, available bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.controllers {
		if f.controllers[i].ClassName == class {
			f.controllers[i].Availability = available
		}
	}
}

const ingressControllersConfig = `
resource "portainer_kubernetes_ingresscontrollers" "c" {
  environment_id = 1
  controllers {
    name         = "nginx"
    class_name   = "nginx"
    type         = "nginx"
    availability = %t
    used         = false
    new          = false
  }
}
`

// TestKubernetesIngressControllers_Lifecycle: the configured list is sent
// after checking the classes exist, other controllers of the cluster and
// the informational fields Portainer fills do not show as a change, drift of
// the availability does, and destroy makes the controllers available again.
func TestKubernetesIngressControllers_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeIngressControllers(mock, "/kubernetes/1/ingresscontrollers")
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(ingressControllersConfig, false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_kubernetes_ingresscontrollers.c", "id", "1"),
					resource.TestCheckResourceAttr("portainer_kubernetes_ingresscontrollers.c", "controllers.#", "1"),
					resource.TestCheckResourceAttr("portainer_kubernetes_ingresscontrollers.c", "controllers.0.used", "false"),
					a2Check(func() error {
						var body []IngressController
						a2LastBody(t, mock, "PUT", "/kubernetes/1/ingresscontrollers", &body)
						want := []IngressController{{Name: "nginx", ClassName: "nginx", Type: "nginx"}}
						if !reflect.DeepEqual(body, want) {
							return fmt.Errorf("payload: %+v", body)
						}
						return nil
					}),
				),
			},
			{Config: mockProviderConfig(mock) + fmt.Sprintf(ingressControllersConfig, false), PlanOnly: true},
			{
				PreConfig:          func() { fake.setAvailability("nginx", true) },
				Config:             mockProviderConfig(mock) + fmt.Sprintf(ingressControllersConfig, false),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{Config: mockProviderConfig(mock) + fmt.Sprintf(ingressControllersConfig, true)},
			{
				ResourceName:     "portainer_kubernetes_ingresscontrollers.c",
				ImportState:      true,
				ImportStateId:    "1",
				ImportStateCheck: ingressControllersImportCheck(2),
			},
		},
		CheckDestroy: a2Check(func() error {
			var body []IngressController
			a2LastBody(t, mock, "PUT", "/kubernetes/1/ingresscontrollers", &body)
			if len(body) != 1 || !body[0].Availability {
				return fmt.Errorf("destroy must make the controllers available again: %+v", body)
			}
			return nil
		}),
	})
}

// ingressControllersImportCheck: an import takes every controller.
func ingressControllersImportCheck(n int) resource.ImportStateCheckFunc {
	return func(states []*terraform.InstanceState) error {
		if len(states) != 1 {
			return fmt.Errorf("expected 1 imported state, got %d", len(states))
		}
		if got := states[0].Attributes["controllers.#"]; got != fmt.Sprint(n) {
			return fmt.Errorf("controllers: expected %d, got %s", n, got)
		}
		return nil
	}
}

func TestKubernetesIngressControllers_Errors(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeIngressControllers(mock, "/kubernetes/1/ingresscontrollers")
	failing := NewMockServer(t)
	failing.On("GET", "/kubernetes/1/ingresscontrollers", RespondString(http.StatusInternalServerError, "", "boom"))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + `
resource "portainer_kubernetes_ingresscontrollers" "c" {
  environment_id = 1
  controllers {
    name         = "haproxy"
    class_name   = "haproxy"
    type         = "custom"
    availability = true
    used         = false
    new          = false
  }
}`,
				ExpectError: regexp.MustCompile(`ingress class "haproxy" is not in the cluster; available classes: nginx,\s+traefik`),
			},
			{
				Config:      mockProviderConfig(failing) + fmt.Sprintf(ingressControllersConfig, true),
				ExpectError: regexp.MustCompile(`failed to read ingress controllers`),
			},
			{
				Config:        mockProviderConfig(mock) + fmt.Sprintf(ingressControllersConfig, true),
				ResourceName:  "portainer_kubernetes_ingresscontrollers.c",
				ImportState:   true,
				ImportStateId: "prod",
				ExpectError:   regexp.MustCompile(`expected <environment_id>`),
			},
		},
	})
	if n := a2Count(mock, "PUT", "/kubernetes/1/ingresscontrollers"); n != 0 {
		t.Errorf("a rejected list must change nothing, got %d PUT", n)
	}
}

func TestKubernetesIngressControllers_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeIngressControllers(mock, "/kubernetes/1/ingresscontrollers")
	a2Upgrade(t, mock, fmt.Sprintf(ingressControllersConfig, false))
}

func TestRefreshIngressControllers(t *testing.T) {
	t.Parallel()
	actual := []IngressController{
		{Name: "nginx", ClassName: "nginx", Type: "nginx", Availability: false, Used: true},
		{Name: "traefik", ClassName: "traefik", Type: "traefik", Availability: true},
	}
	configured := []interface{}{
		map[string]interface{}{"name": "mine", "class_name": "nginx", "type": "custom", "availability": true, "used": false, "new": false},
		map[string]interface{}{"name": "gone", "class_name": "haproxy", "type": "custom", "availability": true, "used": false, "new": false},
	}
	got := refreshIngressControllers(actual, configured)
	want := []map[string]interface{}{{"name": "mine", "class_name": "nginx", "type": "custom", "availability": false, "used": false, "new": false}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("configured: got %v", got)
	}
	if all := refreshIngressControllers(actual, nil); len(all) != 2 || all[1]["class_name"] != "traefik" {
		t.Errorf("import: got %v", all)
	}
}
