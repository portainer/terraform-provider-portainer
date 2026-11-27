package internal

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestKubernetesPodsDataSource_ClusterWide verifies an unset namespace uses
// the cluster-wide route, that readiness is aggregated from the container
// statuses, and that a pod whose kubelet has not reported yet is not ready.
func TestKubernetesPodsDataSource_ClusterWide(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	const ds = "data.portainer_kubernetes_pods.p"
	k8sDSRun(t, mock, `data "portainer_kubernetes_pods" "p" { environment_id = 5 }`,
		resource.TestCheckResourceAttr(ds, "id", "5//pods"),
		resource.TestCheckResourceAttr(ds, "pods.#", "2"),
		resource.TestCheckResourceAttr(ds, "pods.0.name", "web-1"),
		resource.TestCheckResourceAttr(ds, "pods.0.namespace", "default"),
		resource.TestCheckResourceAttr(ds, "pods.0.node_name", "worker-1"),
		resource.TestCheckResourceAttr(ds, "pods.0.phase", "Running"),
		resource.TestCheckResourceAttr(ds, "pods.0.pod_ip", "10.1.0.5"),
		resource.TestCheckResourceAttr(ds, "pods.0.ready", "true"),
		resource.TestCheckResourceAttr(ds, "pods.0.restart_count", "3"),
		resource.TestCheckResourceAttr(ds, "pods.0.containers.#", "2"),
		resource.TestCheckResourceAttr(ds, "pods.0.containers.1.restart_count", "1"),
		resource.TestCheckResourceAttr(ds, "pods.1.ready", "false"),
		resource.TestCheckResourceAttr(ds, "pods.1.labels.%", "0"),
		k8sDSRequest(mock, "GET", "/kubernetes/5/pods", k8sDSQueryIs("")),
	)
}

// TestKubernetesPodsDataSource_NamespacedWithSelectors verifies a configured
// namespace switches to the namespaced route and that both selectors reach
// the API as query parameters.
func TestKubernetesPodsDataSource_NamespacedWithSelectors(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	k8sDSRun(t, mock, `
data "portainer_kubernetes_pods" "p" {
  environment_id = 5
  namespace      = "prod"
  label_selector = "app=web"
  field_selector = "status.phase=Running"
}`,
		resource.TestCheckResourceAttr("data.portainer_kubernetes_pods.p", "id", "5/prod/pods"),
		resource.TestCheckResourceAttr("data.portainer_kubernetes_pods.p", "pods.#", "0"),
		k8sDSRequest(mock, "GET", "/kubernetes/5/namespaces/prod/pods",
			k8sDSQueryIs("fieldSelector=status.phase%3DRunning&labelSelector=app%3Dweb")),
	)
}

// TestKubernetesPodLogsDataSource_HappyPath verifies the plain-text log body
// is stored verbatim and that the optional parameters reach the API - and
// never follow, which would hold the response open during a plan.
func TestKubernetesPodLogsDataSource_HappyPath(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	const ds = "data.portainer_kubernetes_pod_logs.l"
	k8sDSRun(t, mock, `
data "portainer_kubernetes_pod_logs" "l" {
  environment_id = 6
  namespace      = "default"
  pod_name       = "web-1"
  container      = "web"
  tail_lines     = 200
  since_seconds  = 3600
  timestamps     = true
  previous       = true
}`,
		resource.TestCheckResourceAttr(ds, "logs", "2026-09-01T10:00:00Z starting\n2026-09-01T10:00:01Z ready\n"),
		resource.TestCheckResourceAttr(ds, "line_count", "2"),
		resource.TestCheckResourceAttr(ds, "id", "6/default/web-1/log"),
		k8sDSRequest(mock, "GET", "/kubernetes/6/namespaces/default/pods/web-1/log", func(r *RecordedRequest) error {
			if err := k8sDSQueryHas("container=web", "tailLines=200", "sinceSeconds=3600", "timestamps=true", "previous=true")(r); err != nil {
				return err
			}
			if strings.Contains(r.Query, "follow") {
				return fmt.Errorf("query must never request follow, got %q", r.Query)
			}
			return nil
		}),
	)
}

// TestKubernetesPodLogsDataSource_OmitsUnsetOptions verifies that unset
// options are left out entirely, so Kubernetes applies its own defaults, and
// that the SDKv2 defaults of the two flags are what the state holds.
func TestKubernetesPodLogsDataSource_OmitsUnsetOptions(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/6/namespaces/default/pods/web-1/log", RespondString(http.StatusOK, "text/plain", ""))
	const ds = "data.portainer_kubernetes_pod_logs.l"
	k8sDSRun(t, mock, `
data "portainer_kubernetes_pod_logs" "l" {
  environment_id = 6
  namespace      = "default"
  pod_name       = "web-1"
}`,
		resource.TestCheckResourceAttr(ds, "line_count", "0"),
		resource.TestCheckResourceAttr(ds, "timestamps", "false"),
		resource.TestCheckResourceAttr(ds, "previous", "false"),
		resource.TestCheckNoResourceAttr(ds, "tail_lines"),
		k8sDSRequest(mock, "GET", "/kubernetes/6/namespaces/default/pods/web-1/log", k8sDSQueryIs("")),
	)
}

// TestKubernetesPodLogsDataSource_ErrorStatus verifies a non-2xx response is
// reported as an error rather than stored as if it were log output.
func TestKubernetesPodLogsDataSource_ErrorStatus(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/6/namespaces/default/pods/web-1/log",
		RespondString(http.StatusForbidden, "application/json", `{"message":"Unauthorized access to the Kubernetes API"}`))
	k8sDSRunError(t, mock, `
data "portainer_kubernetes_pod_logs" "l" {
  environment_id = 6
  namespace      = "default"
  pod_name       = "web-1"
}`, `failed to read logs of pod "web-1" in namespace "default": status 403`)
}

// TestKubernetesPodLogsDataSource_Validation keeps IntAtLeast(1).
func TestKubernetesPodLogsDataSource_Validation(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	k8sDSRunError(t, mock, `
data "portainer_kubernetes_pod_logs" "l" {
  environment_id = 6
  namespace      = "default"
  pod_name       = "web-1"
  tail_lines     = 0
}`, `at least 1`)
}

// TestKubernetesDeploymentsDataSource_ClusterWide verifies an unset namespace
// uses the cluster-wide route and that spec and status are both mapped.
func TestKubernetesDeploymentsDataSource_ClusterWide(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	const ds = "data.portainer_kubernetes_deployments.d"
	checks := make([]resource.TestCheckFunc, 0, 32)
	checks = append(checks,
		resource.TestCheckResourceAttr(ds, "deployments.#", "1"),
		resource.TestCheckResourceAttr(ds, "deployments.0.images.#", "2"),
		resource.TestCheckResourceAttr(ds, "deployments.0.images.0", "nginx:1.27"),
		resource.TestCheckResourceAttr(ds, "id", "8//deployments"),
	)
	for field, want := range map[string]string{
		"name": "web", "namespace": "default",
		"replicas": "3", "ready_replicas": "2", "available_replicas": "2",
		"updated_replicas": "3", "unavailable_replicas": "1",
		"generation": "4", "observed_generation": "3",
		"creation_timestamp": "2026-08-01T09:00:00Z",
	} {
		checks = append(checks, resource.TestCheckResourceAttr(ds, "deployments.0."+field, want))
	}
	k8sDSRun(t, mock, `data "portainer_kubernetes_deployments" "d" { environment_id = 8 }`, checks...)
}

// TestKubernetesDeploymentsDataSource_MissingReplicasDefaultsToOne covers the
// Kubernetes pointer semantics of spec.replicas: absent means 1, not 0.
func TestKubernetesDeploymentsDataSource_MissingReplicasDefaultsToOne(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	const ds = "data.portainer_kubernetes_deployments.d"
	k8sDSRun(t, mock, `
data "portainer_kubernetes_deployments" "d" {
  environment_id = 8
  namespace      = "default"
}`,
		resource.TestCheckResourceAttr(ds, "deployments.0.replicas", "1"),
		resource.TestCheckResourceAttr(ds, "deployments.0.labels.%", "0"),
		resource.TestCheckResourceAttr(ds, "id", "8/default/deployments"),
	)
}

// TestKubernetesDeploymentsDataSource_Selectors verifies both selectors reach
// the API as query parameters.
func TestKubernetesDeploymentsDataSource_Selectors(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	k8sDSRun(t, mock, `
data "portainer_kubernetes_deployments" "d" {
  environment_id = 8
  label_selector = "app=web"
  field_selector = "metadata.name=web"
}`,
		k8sDSRequest(mock, "GET", "/kubernetes/8/deployments",
			k8sDSQueryIs("fieldSelector=metadata.name%3Dweb&labelSelector=app%3Dweb")),
	)
}

// TestKubernetesReplicaSetsDataSource_Revision verifies the rollout revision
// is read from the deployment.kubernetes.io/revision annotation, and that a
// missing or unparsable one reads as 0 with the replica default of 1.
func TestKubernetesReplicaSetsDataSource_Revision(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	const ds = "data.portainer_kubernetes_replicasets.r"
	k8sDSRun(t, mock, `
data "portainer_kubernetes_replicasets" "r" {
  environment_id = 9
  namespace      = "default"
}`,
		resource.TestCheckResourceAttr(ds, "replicasets.0.revision", "7"),
		resource.TestCheckResourceAttr(ds, "replicasets.0.name", "web-5d9f"),
		resource.TestCheckResourceAttr(ds, "replicasets.0.fully_labeled_replicas", "3"),
		resource.TestCheckResourceAttr(ds, "replicasets.1.revision", "0"),
		resource.TestCheckResourceAttr(ds, "replicasets.1.replicas", "1"),
		resource.TestCheckResourceAttr(ds, "id", "9/default/replicasets"),
		k8sDSRequest(mock, "GET", "/kubernetes/9/namespaces/default/replicasets", k8sDSQueryIs("")),
	)
}

// TestKubernetesReplicaSetsDataSource_DeploymentFilter verifies the
// deployment filter is passed through as its own query parameter alongside
// the selectors.
func TestKubernetesReplicaSetsDataSource_DeploymentFilter(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	k8sDSRun(t, mock, `
data "portainer_kubernetes_replicasets" "r" {
  environment_id = 9
  namespace      = "default"
  deployment     = "web"
  label_selector = "app=web"
}`,
		k8sDSRequest(mock, "GET", "/kubernetes/9/namespaces/default/replicasets",
			k8sDSQueryHas("deployment=web", "labelSelector=app%3Dweb")),
	)
}

// TestKubernetesResourceQuotasDataSource verifies the namespaced route, that
// limits and usage arrive as Kubernetes quantity strings, and that before the
// quota controller has observed an object the spec limit is used.
func TestKubernetesResourceQuotasDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	const ds = "data.portainer_kubernetes_resource_quotas.q"
	k8sDSRun(t, mock, `
data "portainer_kubernetes_resource_quotas" "q" {
  environment_id = 2
  namespace      = "prod"
}`,
		resource.TestCheckResourceAttr(ds, "resource_quotas.#", "2"),
		resource.TestCheckResourceAttr(ds, "resource_quotas.0.name", "compute"),
		resource.TestCheckResourceAttr(ds, "resource_quotas.0.namespace", "prod"),
		resource.TestCheckResourceAttr(ds, "resource_quotas.0.hard.limits.cpu", "4"),
		resource.TestCheckResourceAttr(ds, "resource_quotas.0.hard.limits.memory", "8Gi"),
		resource.TestCheckResourceAttr(ds, "resource_quotas.0.used.limits.cpu", "500m"),
		resource.TestCheckResourceAttr(ds, "resource_quotas.0.scopes.0", "NotTerminating"),
		resource.TestCheckResourceAttr(ds, "resource_quotas.1.hard.pods", "10"),
		resource.TestCheckResourceAttr(ds, "resource_quotas.1.used.%", "0"),
		resource.TestCheckResourceAttr(ds, "resource_quotas.1.scopes.#", "0"),
		resource.TestCheckResourceAttr(ds, "id", "2/prod/resource_quotas"),
	)
}
