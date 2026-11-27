package internal

import (
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// fakeNamespaceSystem serves the system flag of namespace team-a.
func fakeNamespaceSystem(mock *MockServer) *a2Store {
	s := newA2Store()
	s.put("team-a", map[string]interface{}{"Name": "team-a", "IsSystem": false})
	s.serve(mock, "/kubernetes/1/namespaces/team-a", "team-a")
	mock.On("PUT", "/kubernetes/1/namespaces/team-a/system", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]bool
		a2Decode(r, &body)
		s.put("team-a", map[string]interface{}{"Name": "team-a", "IsSystem": body["system"]})
		w.WriteHeader(http.StatusNoContent)
	})
	return s
}

const namespaceSystemConfig = `
resource "portainer_kubernetes_namespace_system" "s" {
  environment_id = 1
  namespace      = %q
  system         = %t
}
`

// TestKubernetesNamespaceSystem_Lifecycle: the flag is put and read back,
// changed in place, imported from "<environment_id>:<namespace>", and
// destroy unmarks the namespace.
func TestKubernetesNamespaceSystem_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	s := fakeNamespaceSystem(mock)
	flag := func(want bool) resource.TestCheckFunc {
		return a2Check(func() error {
			var body map[string]interface{}
			a2LastBody(t, mock, "PUT", "/kubernetes/1/namespaces/team-a/system", &body)
			if body["system"] != want {
				return fmt.Errorf("payload: %v", body)
			}
			return nil
		})
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(namespaceSystemConfig, "team-a", true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_kubernetes_namespace_system.s", "id", "1:team-a"),
					flag(true),
				),
			},
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(namespaceSystemConfig, "team-a", false),
				Check:  flag(false),
			},
			{
				ResourceName:      "portainer_kubernetes_namespace_system.s",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				PreConfig: func() {
					s.put("team-a", map[string]interface{}{"Name": "team-a", "IsSystem": true})
				},
				Config:             mockProviderConfig(mock) + fmt.Sprintf(namespaceSystemConfig, "team-a", false),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{Config: mockProviderConfig(mock) + fmt.Sprintf(namespaceSystemConfig, "team-a", true)},
		},
		CheckDestroy: flag(false),
	})
}

func TestKubernetesNamespaceSystem_Errors(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	s := fakeNamespaceSystem(mock)
	failing := NewMockServer(t)
	failing.On("PUT", "/kubernetes/1/namespaces/team-a/system", RespondString(http.StatusForbidden, "application/json", `{"message":"forbidden"}`))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config:      mockProviderConfig(mock) + fmt.Sprintf(namespaceSystemConfig, "default", true),
				ExpectError: regexp.MustCompile(`the default namespace cannot be a system namespace`),
			},
			{
				Config:      mockProviderConfig(failing) + fmt.Sprintf(namespaceSystemConfig, "team-a", true),
				ExpectError: regexp.MustCompile(`failed to toggle namespace system state`),
			},
			{Config: mockProviderConfig(mock) + fmt.Sprintf(namespaceSystemConfig, "team-a", true)},
			{
				PreConfig:          func() { s.del("team-a") },
				Config:             mockProviderConfig(mock) + fmt.Sprintf(namespaceSystemConfig, "team-a", true),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config:        mockProviderConfig(mock) + fmt.Sprintf(namespaceSystemConfig, "team-a", true),
				ResourceName:  "portainer_kubernetes_namespace_system.s",
				ImportState:   true,
				ImportStateId: "team-a",
				ExpectError:   regexp.MustCompile(`invalid ID format, expected 'envID:namespace'`),
			},
		},
	})
}

func TestKubernetesNamespaceSystem_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fakeNamespaceSystem(mock)
	a2Upgrade(t, mock, fmt.Sprintf(namespaceSystemConfig, "team-a", true))
}
