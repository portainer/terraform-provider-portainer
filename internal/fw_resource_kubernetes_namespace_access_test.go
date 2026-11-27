package internal

import (
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const namespaceAccessConfig = `
resource "portainer_kubernetes_namespace_access" "a" {
  endpoint_id  = 2
  namespace_id = %q
  %s
}
`

// TestKubernetesNamespaceAccess_ResolvesNamespaceName: a namespace name is
// looked up, the four lists are always sent (empty when unset), and the
// change is applied again in place.
func TestKubernetesNamespaceAccess_ResolvesNamespaceName(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/2/namespaces", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Name": "team-a", "Id": "uid-a"}, {"Name": "team-b", "Id": "uid-b"},
	}))
	mock.On("PUT", "/endpoints/2/pools/team-b/access", RespondString(http.StatusNoContent, "", ""))
	body := func(want map[string]interface{}) resource.TestCheckFunc {
		return a2Check(func() error {
			var payload map[string]interface{}
			a2LastBody(t, mock, "PUT", "/endpoints/2/pools/team-b/access", &payload)
			if !reflect.DeepEqual(payload, want) {
				return fmt.Errorf("payload: %v", payload)
			}
			return nil
		})
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(namespaceAccessConfig, "team-b", "users_to_add = [3, 4]"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_kubernetes_namespace_access.a", "id", "2/team-b"),
					body(map[string]interface{}{
						"usersToAdd": []interface{}{float64(3), float64(4)}, "usersToRemove": []interface{}{},
						"teamsToAdd": []interface{}{}, "teamsToRemove": []interface{}{},
					}),
				),
			},
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(namespaceAccessConfig, "team-b", "teams_to_remove = [7]"),
				Check: body(map[string]interface{}{
					"usersToAdd": []interface{}{}, "usersToRemove": []interface{}{},
					"teamsToAdd": []interface{}{}, "teamsToRemove": []interface{}{float64(7)},
				}),
			},
		},
	})
}

func TestKubernetesNamespaceAccess_ColonSkipsLookupAndErrors(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("PUT", "/endpoints/2/pools/ns:1234/access", RespondString(http.StatusNoContent, "", ""))
	mock.On("GET", "/kubernetes/2/namespaces", RespondJSON(http.StatusOK, []map[string]interface{}{{"Name": "team-a"}}))
	mock.On("PUT", "/endpoints/2/pools/team-a/access", RespondString(http.StatusForbidden, "", "access denied"))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(namespaceAccessConfig, "ns:1234", ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_kubernetes_namespace_access.a", "id", "2/ns:1234"),
					a2Check(func() error {
						if mock.FindRequest("GET", "/kubernetes/2/namespaces") != nil {
							return fmt.Errorf("a namespace_id with a colon must not be looked up")
						}
						return nil
					}),
				),
			},
			{
				Config:      mockProviderConfig(mock) + fmt.Sprintf(namespaceAccessConfig, "missing", ""),
				ExpectError: regexp.MustCompile(`namespace missing not found`),
			},
			{
				Config:      mockProviderConfig(mock) + fmt.Sprintf(namespaceAccessConfig, "team-a", ""),
				ExpectError: regexp.MustCompile(`failed to update namespace access: access denied`),
			},
		},
	})
}

func TestKubernetesNamespaceAccess_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("PUT", "/endpoints/2/pools/ns:1/access", RespondString(http.StatusNoContent, "", ""))
	a2Upgrade(t, mock, fmt.Sprintf(namespaceAccessConfig, "ns:1", ""), fmt.Sprintf(namespaceAccessConfig, "ns:1", "users_to_add = [3]\n  teams_to_remove = [1]"))
}

func TestContainsColon(t *testing.T) {
	t.Parallel()
	for s, want := range map[string]bool{"ns:1": true, "team-a": false, "": false} {
		if got := containsColon(s); got != want {
			t.Errorf("containsColon(%q) = %v", s, got)
		}
	}
}
