package internal

import (
	"fmt"
	"net/http"
	"regexp"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// fakeHelm serves the release my-nginx of environment 3.
func fakeHelm(mock *MockServer) *a2Store {
	s := newA2Store()
	mock.On("POST", "/endpoints/3/kubernetes/helm", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		a2Decode(r, &body)
		s.put("my-nginx", map[string]interface{}{"name": body["name"], "namespace": body["namespace"]})
		RespondJSON(http.StatusCreated, map[string]interface{}{})(w, r)
	})
	s.serve(mock, "/endpoints/3/kubernetes/helm/my-nginx", "my-nginx")
	mock.On("DELETE", "/endpoints/3/kubernetes/helm/my-nginx", func(w http.ResponseWriter, r *http.Request) {
		s.del("my-nginx")
		w.WriteHeader(http.StatusNoContent)
	})
	return s
}

const helmConfig = `
resource "portainer_kubernetes_helm" "h" {
  environment_id = 3
  chart          = "nginx"
  name           = "my-nginx"
  namespace      = "web"
  repo           = "https://charts.bitnami.com/bitnami"
  %s
}
`

// TestKubernetesHelm_Lifecycle: the install payload, the ID, a release
// removed outside Terraform planned again, import (with chart/repo/values to
// be configured) and the uninstall with the namespace in the query.
func TestKubernetesHelm_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	s := fakeHelm(mock)
	config := mockProviderConfig(mock) + fmt.Sprintf(helmConfig, "timeouts {\n    create = \"20m\"\n  }")
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_kubernetes_helm.h", "id", "3:web:my-nginx"),
					resource.TestCheckResourceAttr("portainer_kubernetes_helm.h", "values", ""),
					a2Check(func() error {
						var body map[string]interface{}
						a2LastBody(t, mock, "POST", "/endpoints/3/kubernetes/helm", &body)
						want := map[string]interface{}{"chart": "nginx", "name": "my-nginx", "namespace": "web", "repo": "https://charts.bitnami.com/bitnami", "values": ""}
						if fmt.Sprint(body) != fmt.Sprint(want) {
							return fmt.Errorf("payload: %v", body)
						}
						if r := mock.FindRequest("GET", "/endpoints/3/kubernetes/helm/my-nginx"); r == nil || r.Query != "namespace=web" {
							return fmt.Errorf("read query: %+v", r)
						}
						return nil
					}),
				),
			},
			{
				// The timeouts change in place.
				Config: mockProviderConfig(mock) + fmt.Sprintf(helmConfig, ""),
			},
			{
				ResourceName:            "portainer_kubernetes_helm.h",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"chart", "repo", "timeouts"},
			},
			{
				PreConfig:          func() { s.del("my-nginx") },
				Config:             mockProviderConfig(mock) + fmt.Sprintf(helmConfig, ""),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{Config: mockProviderConfig(mock) + fmt.Sprintf(helmConfig, "")},
		},
		CheckDestroy: a2Check(func() error {
			var del *RecordedRequest
			for _, r := range mock.Requests() {
				if r.Method == http.MethodDelete {
					del = r
				}
			}
			if del == nil || del.Path != "/endpoints/3/kubernetes/helm/my-nginx" || del.Query != "namespace=web" {
				return fmt.Errorf("uninstall request: %+v", del)
			}
			return nil
		}),
	})
}

func TestKubernetesHelm_Errors(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/endpoints/3/kubernetes/helm", RespondString(http.StatusBadRequest, "application/json", `{"message":"chart not found"}`))
	mock.On("GET", "/endpoints/3/kubernetes/helm/my-nginx", RespondString(http.StatusInternalServerError, "", "boom"))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config:      mockProviderConfig(mock) + fmt.Sprintf(helmConfig, ""),
				ExpectError: regexp.MustCompile(`failed to install helm chart`),
			},
			{
				Config:        mockProviderConfig(mock) + fmt.Sprintf(helmConfig, ""),
				ResourceName:  "portainer_kubernetes_helm.h",
				ImportState:   true,
				ImportStateId: "3:web:my-nginx",
				ExpectError:   regexp.MustCompile(`failed to read helm release my-nginx`),
			},
			{
				Config:        mockProviderConfig(mock) + fmt.Sprintf(helmConfig, ""),
				ResourceName:  "portainer_kubernetes_helm.h",
				ImportState:   true,
				ImportStateId: "0:web:my-nginx",
				ExpectError:   regexp.MustCompile(`expected 'envID:namespace:release'`),
			},
		},
	})

	// The uninstall answers anything but 204 with an error, once.
	failing := NewMockServer(t)
	var failOnce atomic.Bool
	failing.On("DELETE", "/endpoints/3/kubernetes/helm/my-nginx", func(w http.ResponseWriter, r *http.Request) {
		if failOnce.CompareAndSwap(true, false) {
			RespondJSON(http.StatusOK, map[string]interface{}{})(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	fakeHelm(failing)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: mockProviderConfig(failing) + fmt.Sprintf(helmConfig, "")},
			{
				PreConfig:   func() { failOnce.Store(true) },
				Config:      mockProviderConfig(failing) + fmt.Sprintf(helmConfig, ""),
				Destroy:     true,
				ExpectError: regexp.MustCompile(`failed to delete helm release`),
			},
		},
	})
}

func TestKubernetesHelm_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fakeHelm(mock)
	a2Upgrade(t, mock, fmt.Sprintf(helmConfig, ""), fmt.Sprintf(helmConfig, `values = "replicaCount: 2"`+"\n  timeouts {\n    create = \"20m\"\n  }"))
}
