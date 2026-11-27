package internal

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestKubernetesClusterDataSource folds the four cluster endpoints into one
// data source: the bare JSON boolean of the RBAC endpoint, both dashboard
// shapes (the object the server returns, the list the specification
// describes) and an empty dashboard, whose counters stay unset as they did.
func TestKubernetesClusterDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	const full, empty = "data.portainer_kubernetes_cluster.full", "data.portainer_kubernetes_cluster.empty"
	config := `
data "portainer_kubernetes_cluster" "full" { environment_id = 4 }
data "portainer_kubernetes_cluster" "empty" { environment_id = 1 }`

	checks := make([]resource.TestCheckFunc, 0, 32)
	checks = append(checks,
		resource.TestCheckResourceAttr(full, "id", "4/cluster"),
		resource.TestCheckResourceAttr(empty, "rbac_enabled", "false"),
		resource.TestCheckResourceAttr(empty, "max_cpu", "0"),
		resource.TestCheckNoResourceAttr(empty, "namespaces_count"),
	)
	for field, want := range map[string]string{
		"version": "v1.31.4+k3s1", "major": "1", "minor": "31",
		"platform": "linux/amd64", "supports_pod_restart": "true", "rbac_enabled": "true",
		"applications_count": "3", "namespaces_count": "4", "services_count": "5",
		"ingresses_count": "1", "config_maps_count": "6", "secrets_count": "7", "volumes_count": "2",
		"max_cpu": "3800", "max_memory": "7516192768",
	} {
		checks = append(checks, resource.TestCheckResourceAttr(full, field, want))
	}
	k8sDSRun(t, mock, config, checks...)

	// The list the specification describes.
	listOnly := NewMockServer(t)
	listOnly.On("GET", "/kubernetes/4/dashboard", RespondJSON(200, []map[string]interface{}{
		{"applicationsCount": 3, "namespacesCount": 4, "servicesCount": 5, "ingressesCount": 1, "configMapsCount": 6, "secretsCount": 7, "volumesCount": 2},
	}))
	listOnly.On("GET", "/kubernetes/4/version", RespondJSON(200, map[string]interface{}{"gitVersion": "v1.31.0", "major": "1", "minor": "31"}))
	listOnly.On("GET", "/kubernetes/4/rbac_enabled", RespondJSON(200, true))
	listOnly.On("GET", "/kubernetes/4/max_resource_limits", RespondJSON(200, map[string]interface{}{"CPU": 4000, "Memory": 8192}))
	k8sDSRun(t, listOnly, `data "portainer_kubernetes_cluster" "full" { environment_id = 4 }`,
		resource.TestCheckResourceAttr(full, "applications_count", "3"),
		resource.TestCheckResourceAttr(full, "volumes_count", "2"),
	)
}

// TestKubernetesNodesDataSource verifies the node list, Portainer's per-node
// limits and the metrics server are joined by node name.
func TestKubernetesNodesDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	const ds = "data.portainer_kubernetes_nodes.n"
	checks := make([]resource.TestCheckFunc, 0, 32)
	checks = append(checks,
		resource.TestCheckResourceAttr(ds, "nodes.#", "2"),
		resource.TestCheckResourceAttr(ds, "nodes.1.ready", "false"),
		resource.TestCheckResourceAttr(ds, "nodes.1.usage_cpu", ""),
		resource.TestCheckResourceAttr(ds, "id", "1/nodes"),
	)
	for field, want := range map[string]string{
		"name": "worker-1", "ready": "true", "unschedulable": "true",
		"kubelet_version": "v1.31.4", "internal_ip": "10.0.0.5",
		"capacity_cpu": "4", "available_cpu": "3500", "available_memory": "6442450944",
		"usage_cpu": "250m", "usage_memory": "1Gi",
	} {
		checks = append(checks, resource.TestCheckResourceAttr(ds, "nodes.0."+field, want))
	}
	k8sDSRun(t, mock, `data "portainer_kubernetes_nodes" "n" { environment_id = 1 }`, checks...)
}

// TestKubernetesNodesDataSource_WithoutMetricsServer verifies a cluster with
// no metrics-server still reads: usage is reported as unknown.
func TestKubernetesNodesDataSource_WithoutMetricsServer(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/1/nodes", RespondJSON(200, []map[string]interface{}{
		{
			"metadata": map[string]interface{}{"name": "worker-1"},
			"status":   map[string]interface{}{"conditions": []map[string]interface{}{{"type": "Ready", "status": "False"}}},
		},
	}))
	mock.On("GET", "/kubernetes/1/nodes_limits", RespondJSON(200, map[string]interface{}{}))
	mock.On("GET", "/kubernetes/1/metrics/nodes",
		RespondString(500, "application/json", `{"message":"metrics API not available"}`))
	const ds = "data.portainer_kubernetes_nodes.n"
	k8sDSRun(t, mock, `data "portainer_kubernetes_nodes" "n" { environment_id = 1 }`,
		resource.TestCheckResourceAttr(ds, "nodes.0.usage_cpu", ""),
		resource.TestCheckResourceAttr(ds, "nodes.0.ready", "false"),
	)
}

// TestKubernetesEventsDataSource verifies the warning tally, the namespaced
// route and the resource filter of a cluster-wide query.
func TestKubernetesEventsDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	const ds = "data.portainer_kubernetes_events.e"
	k8sDSRun(t, mock, `
data "portainer_kubernetes_events" "e" {
  environment_id = 1
  namespace      = "prod"
}`,
		resource.TestCheckResourceAttr(ds, "warning_count", "1"),
		resource.TestCheckResourceAttr(ds, "events.0.reason", "FailedScheduling"),
		resource.TestCheckResourceAttr(ds, "events.0.name", "web-1"),
		resource.TestCheckResourceAttr(ds, "events.0.namespace", "prod"),
		resource.TestCheckResourceAttr(ds, "events.0.count", "5"),
		resource.TestCheckResourceAttr(ds, "events.1.uid", ""),
		resource.TestMatchResourceAttr(ds, "id", regexp.MustCompile(`^1/prod/events/\d+$`)),
	)

	clusterWide := NewMockServer(t)
	registerK8sDSFixtures(clusterWide)
	k8sDSRun(t, clusterWide, `
data "portainer_kubernetes_events" "e" {
  environment_id = 1
  resource_id    = "u-1"
}`,
		k8sDSRequest(clusterWide, "GET", "/kubernetes/1/events", k8sDSQueryIs("resourceId=u-1")),
	)
}

// TestKubernetesDescribeDataSource verifies kind, name and namespace reach
// the API as query parameters.
func TestKubernetesDescribeDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	const ds = "data.portainer_kubernetes_describe.d"
	k8sDSRun(t, mock, `
data "portainer_kubernetes_describe" "d" {
  environment_id = 1
  kind           = "pod"
  name           = "web-1"
  namespace      = "prod"
}`,
		resource.TestMatchResourceAttr(ds, "describe", regexp.MustCompile(`Status: Running`)),
		resource.TestCheckResourceAttr(ds, "id", "1/prod/pod/web-1"),
		k8sDSRequest(mock, "GET", "/kubernetes/1/describe", k8sDSQueryHas("kind=pod", "name=web-1", "namespace=prod")),
	)
}

// TestKubernetesPersistentVolumesDataSource verifies the released tally,
// which is the storage a Retain policy has kept around.
func TestKubernetesPersistentVolumesDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	const ds = "data.portainer_kubernetes_persistent_volumes.p"
	k8sDSRun(t, mock, `data "portainer_kubernetes_persistent_volumes" "p" { environment_id = 1 }`,
		resource.TestCheckResourceAttr(ds, "released_count", "1"),
		resource.TestCheckResourceAttr(ds, "persistent_volumes.0.claim_ref", "prod/data"),
		resource.TestCheckResourceAttr(ds, "persistent_volumes.0.capacity", "10Gi"),
		resource.TestCheckResourceAttr(ds, "persistent_volumes.1.access_modes.#", "0"),
		resource.TestCheckResourceAttr(ds, "id", "1/persistent_volumes"),
	)
}

// TestKubernetesConfigDataSource verifies the kubeconfig is read as the YAML
// document it is, that the environment filters reach the API, and that the
// credential it carries is marked sensitive.
func TestKubernetesConfigDataSource(t *testing.T) {
	t.Parallel()
	var resp datasource.SchemaResponse
	newKubernetesConfigDataSource().Schema(context.Background(), datasource.SchemaRequest{}, &resp)
	if !resp.Schema.Attributes["kubeconfig"].IsSensitive() {
		t.Error("kubeconfig carries a bearer token and must be marked Sensitive")
	}

	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	const ds = "data.portainer_kubernetes_config.c"
	k8sDSRun(t, mock, `
data "portainer_kubernetes_config" "c" {
  environment_ids         = [1, 2]
  exclude_environment_ids = [3]
}`,
		resource.TestCheckResourceAttr(ds, "kubeconfig", "apiVersion: v1\nkind: Config\nclusters: []\n"),
		resource.TestMatchResourceAttr(ds, "id", regexp.MustCompile(`^kubernetes-config-\d+$`)),
		k8sDSRequest(mock, "GET", "/kubernetes/config", k8sDSQueryHas("ids=1", "ids=2", "excludeIds=3")),
	)
}

// TestKubernetesPodMetricsDataSource verifies both endpoints: the namespace
// one returns a list, the single-pod one a bare object reported as a
// one-element list.
func TestKubernetesPodMetricsDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	const ns, one = "data.portainer_kubernetes_pod_metrics.ns", "data.portainer_kubernetes_pod_metrics.one"
	k8sDSRun(t, mock, `
data "portainer_kubernetes_pod_metrics" "ns" {
  environment_id = 1
  namespace      = "prod"
}
data "portainer_kubernetes_pod_metrics" "one" {
  environment_id = 1
  namespace      = "prod"
  pod            = "web-1"
}`,
		resource.TestCheckResourceAttr(ns, "pods.0.name", "web-1"),
		resource.TestCheckResourceAttr(ns, "pods.0.containers.0.cpu", "250m"),
		resource.TestCheckResourceAttr(ns, "pods.0.containers.0.memory", "1Gi"),
		resource.TestCheckResourceAttr(ns, "id", "1/prod//pod-metrics"),
		resource.TestCheckResourceAttr(one, "pods.#", "1"),
		resource.TestCheckResourceAttr(one, "pods.0.containers.0.cpu", "300m"),
		resource.TestCheckResourceAttr(one, "id", "1/prod/web-1/pod-metrics"),
	)
}

// TestKubernetesNodeMetricsDataSource covers the per-node endpoint.
func TestKubernetesNodeMetricsDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	const ds = "data.portainer_kubernetes_node_metrics.n"
	k8sDSRun(t, mock, `
data "portainer_kubernetes_node_metrics" "n" {
  environment_id = 1
  node           = "worker-1"
}`,
		resource.TestCheckResourceAttr(ds, "cpu", "1200m"),
		resource.TestCheckResourceAttr(ds, "memory", "3Gi"),
		resource.TestCheckResourceAttr(ds, "window", "30s"),
		resource.TestCheckResourceAttr(ds, "id", "1/worker-1/node-metrics"),
	)
}

// TestKubernetesApplicationResourcesDataSource covers the cluster-wide
// request/limit totals, whose JSON keys are PascalCase.
func TestKubernetesApplicationResourcesDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	const ds = "data.portainer_kubernetes_application_resources.r"
	k8sDSRun(t, mock, `data "portainer_kubernetes_application_resources" "r" { environment_id = 1 }`,
		resource.TestCheckResourceAttr(ds, "cpu_request", "2.5"),
		resource.TestCheckResourceAttr(ds, "cpu_limit", "4"),
		resource.TestCheckResourceAttr(ds, "memory_limit", "4294967296"),
		resource.TestCheckResourceAttr(ds, "id", "1/application-resources"),
	)
}

// TestAccKubernetesDataSources_ReadCluster reads a few of the data sources
// from a real Kubernetes environment, with v3.0.0 and with this build, and
// requires the same state from both.
func TestAccKubernetesDataSources_ReadCluster(t *testing.T) {
	accPreCheck(t)
	envID := os.Getenv("PORTAINER_ACC_K8S_ENV_ID")
	if envID == "" {
		t.Skip("PORTAINER_ACC_K8S_ENV_ID must name a Kubernetes environment for this acceptance test")
	}
	config := accProvider() + fmt.Sprintf(`
data "portainer_kubernetes_cluster" "c" { environment_id = %[1]s }
data "portainer_kubernetes_storage_classes" "s" { endpoint_id = %[1]s }
data "portainer_kubernetes_resource_counts" "r" { endpoint_id = %[1]s }
`, envID)

	v300 := map[string]map[string]string{}
	resource.Test(t, resource.TestCase{
		Steps: []resource.TestStep{
			{ExternalProviders: previousRelease, Config: config, Check: k8sDSCaptureState(v300)},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.portainer_kubernetes_cluster.c", "id", envID+"/cluster"),
					func(s *terraform.State) error {
						current := map[string]map[string]string{}
						if err := k8sDSCaptureState(current)(s); err != nil {
							return err
						}
						return k8sDSCompareStates(v300, current)
					},
				),
			},
		},
	})
}
