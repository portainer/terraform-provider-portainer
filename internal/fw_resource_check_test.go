package internal

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func checkMockStandalone(mock *MockServer, state, image string) {
	mock.On("GET", "/endpoints/1/docker/swarm", RespondString(http.StatusNotFound, "application/json", `{"message":"not a swarm"}`))
	mock.On("GET", "/endpoints/1/docker/containers/json", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Names": []interface{}{"/mystack_web.1"}, "State": state, "Image": image},
	}))
}

const checkConfigNoWait = `
resource "portainer_check" "c" {
  endpoint_id         = 1
  stack_name          = "mystack"
  revision            = "1.25"
  services_list       = "web"
  wait                = 0
  wait_between_checks = 0
  max_retries         = 1
}
`

// TestCheckResource_Standalone: no swarm → containers are listed and the
// matching one passes; Read and Delete send nothing.
func TestCheckResource_Standalone(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	checkMockStandalone(mock, "running", "nginx:1.25")

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		CheckDestroy: func(*terraform.State) error {
			for _, r := range mock.Requests() {
				if r.Method != http.MethodGet {
					return fmt.Errorf("unexpected %s %s", r.Method, r.Path)
				}
			}
			return nil
		},
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + checkConfigNoWait,
			Check: resource.ComposeTestCheckFunc(
				resource.TestMatchResourceAttr("portainer_check.c", "id", regexp.MustCompile(`^check-\d+$`)),
				resource.TestCheckResourceAttr("portainer_check.c", "desired_state", "running"),
				resource.TestCheckResourceAttrWith("portainer_check.c", "output", func(v string) error {
					if !strings.Contains(v, "Docker Standalone detected") {
						return fmt.Errorf("output does not mention Docker Standalone: %q", v)
					}
					return nil
				}),
			),
		}},
	})
}

func TestCheckResource_StandaloneRetryFails(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	checkMockStandalone(mock, "exited", "nginx:1.20")

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + checkConfigNoWait,
			ExpectError: regexp.MustCompile(`is not running in revision "1.25"`),
		}},
	})
}

func TestCheckResource_Swarm(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/1/docker/swarm", RespondJSON(http.StatusOK, map[string]interface{}{"ID": "swarm-cluster-id"}))
	mock.On("GET", "/endpoints/1/docker/tasks", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"Spec":   map[string]interface{}{"ContainerSpec": map[string]interface{}{"Image": "nginx:1.25"}},
			"Status": map[string]interface{}{"State": "running"},
		},
	}))

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + checkConfigNoWait,
			Check: resource.TestCheckResourceAttrWith("portainer_check.c", "output", func(v string) error {
				if !strings.Contains(v, "Docker Swarm detected") {
					return fmt.Errorf("output does not mention Docker Swarm: %q", v)
				}
				return nil
			}),
		}},
	})
	req := mock.FindRequest("GET", "/endpoints/1/docker/tasks")
	if req == nil || !strings.Contains(req.Query, "filters=") {
		t.Errorf("expected the tasks to be filtered by service, got %+v", req)
	}
}

// TestCheckResource_UpgradeFromV300: a check applied by v3.0.0, with and
// without the optional arguments, plans no change with this build.
func TestCheckResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	checkMockStandalone(mock, "running", "nginx:1.25")

	config := mockProviderConfig(mock) + checkConfigNoWait + `
resource "portainer_check" "full" {
  endpoint_id         = 1
  stack_name          = "mystack"
  revision            = "1.25"
  services_list       = "web"
  desired_state       = "running"
  wait                = 0
  wait_between_checks = 0
  max_retries         = 2
}
`
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		Steps: []resource.TestStep{
			{ExternalProviders: previousRelease, Config: config},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}
