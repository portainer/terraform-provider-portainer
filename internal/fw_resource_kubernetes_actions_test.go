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

// Tests of the one-shot Kubernetes resources: node drain, deployment
// rollback, object deletion and cluster upgrade.

const nodeDrainConfig = `
resource "portainer_kubernetes_node_drain" "d" {
  environment_id = 4
  node_name      = "worker-1"
  %s
}
`

// TestKubernetesNodeDrain_SendsOptions: every option is sent under
// Portainer's PascalCase names, and the defaults are Portainer's own
// libkubectl.DefaultDrainOptions.
func TestKubernetesNodeDrain_SendsOptions(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/kubernetes/4/nodes/worker-1/drain", RespondString(http.StatusNoContent, "", ""))
	check := func(want map[string]interface{}) resource.TestCheckFunc {
		return a2Check(func() error {
			var payload map[string]interface{}
			a2LastBody(t, mock, "POST", "/kubernetes/4/nodes/worker-1/drain", &payload)
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
				Config: mockProviderConfig(mock) + fmt.Sprintf(nodeDrainConfig, ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_kubernetes_node_drain.d", "id", "4/worker-1"),
					check(map[string]interface{}{
						"Force": false, "TimeoutSeconds": float64(60), "GracePeriodSeconds": float64(-1),
						"IgnoreDaemonSets": true, "DeleteEmptyDirData": true, "DisableEviction": false,
					}),
				),
			},
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(nodeDrainConfig, `
  force                 = true
  timeout_seconds       = 300
  grace_period_seconds  = 30
  delete_empty_dir_data = false
  disable_eviction      = true
`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("portainer_kubernetes_node_drain.d", plancheck.ResourceActionDestroyBeforeCreate),
				}},
				Check: check(map[string]interface{}{
					"Force": true, "TimeoutSeconds": float64(300), "GracePeriodSeconds": float64(30),
					"IgnoreDaemonSets": true, "DeleteEmptyDirData": false, "DisableEviction": true,
				}),
			},
		},
	})
}

func TestKubernetesNodeDrain_Errors(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/kubernetes/4/nodes/worker-1/drain", RespondString(http.StatusInternalServerError, "application/json", `{"message":"cannot evict pod"}`))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config:      mockProviderConfig(mock) + fmt.Sprintf(nodeDrainConfig, ""),
				ExpectError: regexp.MustCompile(`failed to drain Kubernetes node "worker-1"`),
			},
			{
				Config:      mockProviderConfig(mock) + fmt.Sprintf(nodeDrainConfig, "grace_period_seconds = -2"),
				ExpectError: regexp.MustCompile(`at least -1`),
			},
		},
	})
}

func TestKubernetesNodeDrain_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/kubernetes/4/nodes/worker-1/drain", RespondString(http.StatusNoContent, "", ""))
	a2Upgrade(t, mock, fmt.Sprintf(nodeDrainConfig, ""), fmt.Sprintf(nodeDrainConfig, "force = true\n  timeout_seconds = 300"))
}

const deploymentRollbackConfig = `
resource "portainer_kubernetes_deployment_rollback" "r" {
  environment_id  = 2
  namespace       = "default"
  deployment_name = "web"
  %s
}
`

// TestKubernetesDeploymentRollback_SendsRevision: the revision goes out (0,
// the default, is the previous one), the replica count comes back, and the
// ID carries a timestamp so another rollback is a replacement.
func TestKubernetesDeploymentRollback_SendsRevision(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/kubernetes/2/namespaces/default/deployments/web/rollback", RespondJSON(http.StatusOK, map[string]interface{}{
		"spec": map[string]interface{}{"replicas": 4},
	}))
	revision := func(want float64) resource.TestCheckFunc {
		return a2Check(func() error {
			var payload map[string]interface{}
			a2LastBody(t, mock, "POST", "/kubernetes/2/namespaces/default/deployments/web/rollback", &payload)
			if payload["revision"] != want {
				return fmt.Errorf("payload.revision: %v", payload["revision"])
			}
			return nil
		})
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(deploymentRollbackConfig, ""),
				Check: resource.ComposeTestCheckFunc(
					revision(0),
					resource.TestCheckResourceAttr("portainer_kubernetes_deployment_rollback.r", "rolled_back_to_replicas", "4"),
					resource.TestMatchResourceAttr("portainer_kubernetes_deployment_rollback.r", "id", regexp.MustCompile(`^2/default/web/rollback/\d+$`)),
				),
			},
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(deploymentRollbackConfig, "revision = 3"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("portainer_kubernetes_deployment_rollback.r", plancheck.ResourceActionDestroyBeforeCreate),
				}},
				Check: revision(3),
			},
		},
	})
}

func TestKubernetesDeploymentRollback_NoHistory(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/kubernetes/2/namespaces/default/deployments/web/rollback", RespondString(http.StatusBadRequest, "application/json", `{"message":"no rollout history"}`))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + fmt.Sprintf(deploymentRollbackConfig, ""),
			ExpectError: regexp.MustCompile(`failed to roll deployment "web" in namespace "default" back to revision 0`),
		}},
	})
}

func TestKubernetesDeploymentRollback_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/kubernetes/2/namespaces/default/deployments/web/rollback", RespondJSON(http.StatusOK, map[string]interface{}{"spec": map[string]interface{}{}}))
	a2Upgrade(t, mock, fmt.Sprintf(deploymentRollbackConfig, ""), fmt.Sprintf(deploymentRollbackConfig, "revision = 2"))
}

const deleteObjectConfig = `
resource "portainer_kubernetes_delete_object" "d" {
  environment_id = 3
  resource_type  = %q
  namespace      = "default"
  names          = ["web", "api"]
}
`

// TestKubernetesDeleteObject_SendsNamesPerNamespace: the bulk delete takes
// the names per namespace; destroy only forgets the deletion.
func TestKubernetesDeleteObject_SendsNamesPerNamespace(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/kubernetes/3/services/delete", RespondString(http.StatusOK, "application/json", `{}`))
	mock.On("POST", "/kubernetes/3/cluster_roles/delete", RespondString(http.StatusOK, "application/json", `{}`))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(deleteObjectConfig, "services"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_kubernetes_delete_object.d", "id", "3:services:web,api"),
					a2Check(func() error {
						var payload map[string][]string
						a2LastBody(t, mock, "POST", "/kubernetes/3/services/delete", &payload)
						if !reflect.DeepEqual(payload, map[string][]string{"default": {"web", "api"}}) {
							return fmt.Errorf("payload: %v", payload)
						}
						return nil
					}),
				),
			},
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(deleteObjectConfig, "cluster_roles"),
				Check:  resource.TestCheckResourceAttr("portainer_kubernetes_delete_object.d", "id", "3:cluster_roles:web,api"),
			},
		},
	})
	if n := len(mock.Requests()); n != 2 {
		t.Errorf("destroy must only forget the deletion, got %d requests", n)
	}
}

func TestKubernetesDeleteObject_Errors(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/kubernetes/3/services/delete", RespondString(http.StatusForbidden, "application/json", `{"message":"forbidden"}`))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config:      mockProviderConfig(mock) + fmt.Sprintf(deleteObjectConfig, "services"),
				ExpectError: regexp.MustCompile(`failed to delete services`),
			},
			{
				Config:      mockProviderConfig(mock) + fmt.Sprintf(deleteObjectConfig, "pods"),
				ExpectError: regexp.MustCompile(`value must be one of`),
			},
		},
	})
}

func TestKubernetesDeleteObject_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/kubernetes/3/services/delete", RespondString(http.StatusOK, "application/json", `{}`))
	a2Upgrade(t, mock, fmt.Sprintf(deleteObjectConfig, "services"))
}

const clusterUpgradeConfig = `
resource "portainer_kubernetes_cluster_upgrade" "u" {
  endpoint_id = 7
  %s
}
`

// TestKubernetesClusterUpgrade_PostsToEnvironment: the upgrade is posted
// once, and a changed trigger asks for another one.
func TestKubernetesClusterUpgrade_PostsToEnvironment(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/cloud/endpoints/7/upgrade", RespondString(http.StatusNoContent, "", ""))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(clusterUpgradeConfig, `triggers = { v = "1" }`),
				Check:  resource.TestMatchResourceAttr("portainer_kubernetes_cluster_upgrade.u", "id", regexp.MustCompile(`^7-upgrade-\d+$`)),
			},
			{Config: mockProviderConfig(mock) + fmt.Sprintf(clusterUpgradeConfig, `triggers = { v = "1" }`), PlanOnly: true},
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(clusterUpgradeConfig, `triggers = { v = "2" }`),
				Check: a2Check(func() error {
					if n := a2Count(mock, "POST", "/cloud/endpoints/7/upgrade"); n != 2 {
						return fmt.Errorf("expected two upgrades, got %d", n)
					}
					return nil
				}),
			},
		},
	})

	failing := NewMockServer(t)
	failing.On("POST", "/cloud/endpoints/7/upgrade", RespondString(http.StatusConflict, "", "already up to date"))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(failing) + fmt.Sprintf(clusterUpgradeConfig, ""),
			ExpectError: regexp.MustCompile(`failed to upgrade the Kubernetes cluster of environment 7`),
		}},
	})
}

func TestKubernetesClusterUpgrade_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/cloud/endpoints/7/upgrade", RespondString(http.StatusNoContent, "", ""))
	a2Upgrade(t, mock, fmt.Sprintf(clusterUpgradeConfig, ""), fmt.Sprintf(clusterUpgradeConfig, `triggers = { v = "1" }`))
}
