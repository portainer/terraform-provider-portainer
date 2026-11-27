package internal

import (
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

const namespaceIngressControllersConfig = `
resource "portainer_kubernetes_namespace_ingresscontrollers" "c" {
  environment_id = 1
  namespace      = "web"
  controllers {
    name         = "traefik"
    class_name   = "traefik"
    type         = "traefik"
    availability = %t
    used         = false
    new          = true
  }
}
`

// TestKubernetesNamespaceIngressControllers_Lifecycle: the namespace's list
// is put in place (a change updates, it does not replace), drift is
// detected, import takes "<environment_id>:<namespace>" and destroy makes
// the controllers available in the namespace again.
func TestKubernetesNamespaceIngressControllers_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	path := "/kubernetes/1/namespaces/web/ingresscontrollers"
	fake := newFakeIngressControllers(mock, path)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(namespaceIngressControllersConfig, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_kubernetes_namespace_ingresscontrollers.c", "id", "1:web"),
					a2Check(func() error {
						var body []IngressController
						a2LastBody(t, mock, "PUT", path, &body)
						want := []IngressController{{Name: "traefik", ClassName: "traefik", Type: "traefik", Availability: true, New: true}}
						if !reflect.DeepEqual(body, want) {
							return fmt.Errorf("payload: %+v", body)
						}
						return nil
					}),
				),
			},
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(namespaceIngressControllersConfig, false),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("portainer_kubernetes_namespace_ingresscontrollers.c", plancheck.ResourceActionUpdate),
				}},
			},
			{
				PreConfig:          func() { fake.setAvailability("traefik", true) },
				Config:             mockProviderConfig(mock) + fmt.Sprintf(namespaceIngressControllersConfig, false),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{Config: mockProviderConfig(mock) + fmt.Sprintf(namespaceIngressControllersConfig, false)},
			{
				ResourceName:     "portainer_kubernetes_namespace_ingresscontrollers.c",
				ImportState:      true,
				ImportStateId:    "1:web",
				ImportStateCheck: ingressControllersImportCheck(2),
			},
		},
		CheckDestroy: a2Check(func() error {
			var body []IngressController
			a2LastBody(t, mock, "PUT", path, &body)
			if len(body) != 1 || !body[0].Availability {
				return fmt.Errorf("destroy must make the controllers available again: %+v", body)
			}
			return nil
		}),
	})
}

func TestKubernetesNamespaceIngressControllers_Errors(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/1/namespaces/web/ingresscontrollers", RespondString(http.StatusNotFound, "", "namespace not found"))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config:      mockProviderConfig(mock) + fmt.Sprintf(namespaceIngressControllersConfig, true),
				ExpectError: regexp.MustCompile(`failed to read ingress controllers`),
			},
			{
				Config:        mockProviderConfig(mock) + fmt.Sprintf(namespaceIngressControllersConfig, true),
				ResourceName:  "portainer_kubernetes_namespace_ingresscontrollers.c",
				ImportState:   true,
				ImportStateId: "1",
				ExpectError:   regexp.MustCompile(`expected <environment_id>:<namespace>`),
			},
		},
	})
}

func TestKubernetesNamespaceIngressControllers_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeIngressControllers(mock, "/kubernetes/1/namespaces/web/ingresscontrollers")
	a2Upgrade(t, mock, fmt.Sprintf(namespaceIngressControllersConfig, false))
}
