package internal

import (
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// fakeDeployment serves one deployment and its scale subresource.
func fakeDeployment(mock *MockServer, replicas *int) *a2Store {
	s := newA2Store()
	set := func(n *int) {
		spec := map[string]interface{}{}
		if n != nil {
			spec["replicas"] = *n
		}
		ready := 0
		if n != nil {
			ready = *n
		}
		s.put("web", map[string]interface{}{
			"spec":   spec,
			"status": map[string]interface{}{"readyReplicas": ready, "availableReplicas": ready},
		})
	}
	set(replicas)
	s.serve(mock, "/kubernetes/3/namespaces/default/deployments/web", "web")
	mock.On("PUT", "/kubernetes/3/namespaces/default/deployments/web/scale", func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Replicas int }
		a2Decode(r, &body)
		set(&body.Replicas)
		v, _ := s.get("web")
		RespondJSON(http.StatusOK, v)(w, r)
	})
	return s
}

const deploymentScaleConfig = `
resource "portainer_kubernetes_deployment_scale" "s" {
  environment_id  = 3
  namespace       = "default"
  deployment_name = "web"
  replicas        = %d
}
`

// TestKubernetesDeploymentScale_Lifecycle: the scale goes to the scale
// subresource, the status comes from its response, a change scales in place
// (0 included) and destroy leaves the deployment alone.
func TestKubernetesDeploymentScale_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	one := 1
	fakeDeployment(mock, &one)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(deploymentScaleConfig, 3),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_kubernetes_deployment_scale.s", "id", "3/default/web/scale"),
					resource.TestCheckResourceAttr("portainer_kubernetes_deployment_scale.s", "ready_replicas", "3"),
					resource.TestCheckResourceAttr("portainer_kubernetes_deployment_scale.s", "available_replicas", "3"),
					a2Check(func() error {
						var body map[string]interface{}
						a2LastBody(t, mock, "PUT", "/kubernetes/3/namespaces/default/deployments/web/scale", &body)
						if body["replicas"] != float64(3) {
							return fmt.Errorf("payload.replicas: %v", body["replicas"])
						}
						return nil
					}),
				),
			},
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(deploymentScaleConfig, 0),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("portainer_kubernetes_deployment_scale.s", plancheck.ResourceActionUpdate),
				}},
				Check: a2Check(func() error {
					var body map[string]interface{}
					a2LastBody(t, mock, "PUT", "/kubernetes/3/namespaces/default/deployments/web/scale", &body)
					if body["replicas"] != float64(0) {
						return fmt.Errorf("payload.replicas: expected an explicit 0, got %v", body["replicas"])
					}
					return nil
				}),
			},
		},
	})
	if n := a2Count(mock, "DELETE", "/kubernetes/3/namespaces/default/deployments/web"); n != 0 {
		t.Errorf("destroy must not delete the deployment, got %d DELETE", n)
	}
}

// TestKubernetesDeploymentScale_Drift: a replica count changed outside
// Terraform is planned back; a missing spec.replicas is the Kubernetes default
// of 1; a deleted deployment drops the resource.
func TestKubernetesDeploymentScale_Drift(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	one := 1
	s := fakeDeployment(mock, &one)
	config := mockProviderConfig(mock) + fmt.Sprintf(deploymentScaleConfig, 1)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					s.put("web", map[string]interface{}{"spec": map[string]interface{}{"replicas": 7}, "status": map[string]interface{}{}})
				},
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				// No spec.replicas: the default of 1, which the configuration asks for.
				PreConfig: func() {
					s.put("web", map[string]interface{}{"spec": map[string]interface{}{}, "status": map[string]interface{}{}})
				},
				Config:   config,
				PlanOnly: true,
			},
			{
				PreConfig:          func() { s.del("web") },
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestKubernetesDeploymentScale_Errors(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("PUT", "/kubernetes/3/namespaces/default/deployments/web/scale", RespondString(http.StatusNotFound, "application/json", `{"message":"deployment not found"}`))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config:      mockProviderConfig(mock) + fmt.Sprintf(deploymentScaleConfig, -1),
				ExpectError: regexp.MustCompile(`at least 0`),
			},
			{
				Config:      mockProviderConfig(mock) + fmt.Sprintf(deploymentScaleConfig, 2),
				ExpectError: regexp.MustCompile(`failed to scale deployment "web" in namespace "default" to 2 replicas`),
			},
		},
	})
}

func TestKubernetesDeploymentScale_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	one := 1
	fakeDeployment(mock, &one)
	a2Upgrade(t, mock, fmt.Sprintf(deploymentScaleConfig, 2))
}
