package internal

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestKubernetesStorageClassDataSources covers both storage class data
// sources and the map and list fields they flatten.
func TestKubernetesStorageClassDataSources(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	const list, single = "data.portainer_kubernetes_storage_classes.l", "data.portainer_kubernetes_storage_class.s"
	k8sDSRun(t, mock, `
data "portainer_kubernetes_storage_classes" "l" { endpoint_id = 5 }
data "portainer_kubernetes_storage_class" "s" {
  endpoint_id = 5
  name        = "fast-ssd"
}`,
		resource.TestCheckResourceAttr(list, "storage_classes.#", "2"),
		resource.TestCheckResourceAttr(list, "storage_classes.0.name", "fast-ssd"),
		resource.TestCheckResourceAttr(list, "storage_classes.0.is_default", "true"),
		resource.TestCheckResourceAttr(list, "storage_classes.0.parameters.type", "gp3"),
		resource.TestCheckResourceAttr(list, "storage_classes.0.mount_options.#", "1"),
		resource.TestCheckResourceAttr(list, "storage_classes.1.mount_options.#", "0"),
		resource.TestCheckResourceAttr(list, "id", "portainer-kubernetes-storage-classes-5"),
		resource.TestCheckResourceAttr(single, "provisioner_name", "kubernetes.io/aws-ebs"),
		resource.TestCheckResourceAttr(single, "allow_volume_expansion", "true"),
		// The name is an argument, so a response is never allowed to rewrite it.
		resource.TestCheckResourceAttr(single, "name", "fast-ssd"),
		resource.TestCheckResourceAttr(single, "id", "portainer-kubernetes-storage-class-5-fast-ssd"),
	)
}

// TestKubernetesPersistentVolumeClaimsDataSource pins the two listing paths
// Portainer has: one for the whole cluster, one for a single namespace.
func TestKubernetesPersistentVolumeClaimsDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	const cluster, scoped = "data.portainer_kubernetes_persistent_volume_claims.c", "data.portainer_kubernetes_persistent_volume_claims.s"
	k8sDSRun(t, mock, `
data "portainer_kubernetes_persistent_volume_claims" "c" { endpoint_id = 5 }
data "portainer_kubernetes_persistent_volume_claims" "s" {
  endpoint_id = 5
  namespace   = "apps"
}`,
		resource.TestCheckResourceAttr(cluster, "claims.0.phase", "Bound"),
		resource.TestCheckResourceAttr(cluster, "claims.0.storage_request", "10Gi"),
		resource.TestCheckResourceAttr(cluster, "claims.0.storage", "10737418240"),
		resource.TestCheckResourceAttr(cluster, "claims.0.id", "uid-1"),
		resource.TestCheckResourceAttr(cluster, "id", "portainer-kubernetes-pvcs-5-"),
		resource.TestCheckResourceAttr(scoped, "claims.#", "1"),
		resource.TestCheckResourceAttr(scoped, "id", "portainer-kubernetes-pvcs-5-apps"),
		k8sDSRequest(mock, "GET", "/kubernetes/5/persistent_volume_claims", nil),
		k8sDSRequest(mock, "GET", "/kubernetes/5/namespaces/apps/persistent_volume_claims", nil),
	)
}

// TestKubernetesPersistentVolumeClaimDataSource covers the inspect data
// source. Its schema declares "id" as the claim UID, but - as under SDKv2 -
// the stored ID is the data source's own.
func TestKubernetesPersistentVolumeClaimDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/5/namespaces/apps/persistent_volume_claims/data",
		RespondJSON(http.StatusOK, map[string]interface{}{
			"id": "uid-1", "name": "data", "namespace": "apps", "phase": "Pending",
			"storageClass": "fast-ssd", "storageRequest": "10Gi",
			"owningApplications": []string{},
		}))
	const ds = "data.portainer_kubernetes_persistent_volume_claim.c"
	k8sDSRun(t, mock, `
data "portainer_kubernetes_persistent_volume_claim" "c" {
  endpoint_id = 5
  namespace   = "apps"
  name        = "data"
}`,
		resource.TestCheckResourceAttr(ds, "phase", "Pending"),
		resource.TestCheckResourceAttr(ds, "name", "data"),
		resource.TestCheckResourceAttr(ds, "namespace", "apps"),
		resource.TestCheckResourceAttr(ds, "owning_applications.#", "0"),
		resource.TestCheckResourceAttr(ds, "access_modes.#", "0"),
		resource.TestCheckResourceAttr(ds, "id", "portainer-kubernetes-pvc-5-apps-data"),
	)
}

// TestKubernetesVolumesDataSource pins the decision to pass this listing
// through as JSON, the applications flag and the namespaced path.
func TestKubernetesVolumesDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	k8sDSRun(t, mock, `
data "portainer_kubernetes_volumes" "c" {
  endpoint_id       = 5
  with_applications = true
}`,
		resource.TestMatchResourceAttr("data.portainer_kubernetes_volumes.c", "volumes", regexp.MustCompile(`persistentVolumeClaim`)),
		k8sDSRequest(mock, "GET", "/kubernetes/5/volumes", k8sDSQueryHas("withApplications=true")),
	)

	scoped := NewMockServer(t)
	registerK8sDSFixtures(scoped)
	k8sDSRun(t, scoped, `
data "portainer_kubernetes_volumes" "s" {
  endpoint_id = 5
  namespace   = "apps"
}`,
		resource.TestCheckResourceAttr("data.portainer_kubernetes_volumes.s", "with_applications", "false"),
		resource.TestCheckResourceAttr("data.portainer_kubernetes_volumes.s", "id", "portainer-kubernetes-volumes-5-apps"),
		k8sDSRequest(scoped, "GET", "/kubernetes/5/namespaces/apps/volumes", k8sDSQueryIs("")),
	)
}

// TestKubernetesVolumeDataSource covers the single volume view: the fields
// worth acting on are flattened, and the whole response is still available.
func TestKubernetesVolumeDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	const ds = "data.portainer_kubernetes_volume.v"
	k8sDSRun(t, mock, `
data "portainer_kubernetes_volume" "v" {
  endpoint_id = 5
  namespace   = "apps"
  volume      = "data"
}`,
		resource.TestCheckResourceAttr(ds, "persistent_volume_reclaim_policy", "Retain"),
		resource.TestCheckResourceAttr(ds, "claim_storage_request", "10Gi"),
		resource.TestCheckResourceAttr(ds, "storage_class_provisioner", "ebs.csi.aws.com"),
		// The CSI block is not flattened, so it has to survive in the raw details.
		resource.TestMatchResourceAttr(ds, "details", regexp.MustCompile(`"driver":"ebs.csi.aws.com"`)),
	)
}

// TestKubernetesCronJobsDataSource covers the default that keeps Kubernetes'
// own jobs out of a listing a configuration would iterate.
func TestKubernetesCronJobsDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	k8sDSRun(t, mock, `
data "portainer_kubernetes_cron_jobs" "d" { endpoint_id = 5 }
data "portainer_kubernetes_cron_jobs" "s" {
  endpoint_id    = 5
  include_system = true
}`,
		resource.TestCheckResourceAttr("data.portainer_kubernetes_cron_jobs.d", "cron_jobs.#", "1"),
		resource.TestCheckResourceAttr("data.portainer_kubernetes_cron_jobs.d", "cron_jobs.0.name", "backup"),
		resource.TestCheckResourceAttr("data.portainer_kubernetes_cron_jobs.d", "include_system", "false"),
		resource.TestCheckResourceAttr("data.portainer_kubernetes_cron_jobs.s", "cron_jobs.#", "2"),
	)
}

// TestKubernetesEndpointsDataSource covers the nested port list, and the
// empty-address case that identifies a service with no ready backends.
func TestKubernetesEndpointsDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	const ds = "data.portainer_kubernetes_endpoints.e"
	k8sDSRun(t, mock, `data "portainer_kubernetes_endpoints" "e" { endpoint_id = 5 }`,
		resource.TestCheckResourceAttr(ds, "endpoints.#", "3"),
		resource.TestCheckResourceAttr(ds, "endpoints.0.ports.#", "1"),
		resource.TestCheckResourceAttr(ds, "endpoints.0.ports.0.port", "8080"),
		resource.TestCheckResourceAttr(ds, "endpoints.1.addresses.#", "0"),
		resource.TestCheckResourceAttr(ds, "endpoints.2.ports.#", "0"),
	)
}

// TestKubernetesServiceAccountDataSource covers the account inspect,
// including the secret references that are names rather than values.
func TestKubernetesServiceAccountDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	const ds = "data.portainer_kubernetes_service_account.a"
	k8sDSRun(t, mock, `
data "portainer_kubernetes_service_account" "a" {
  endpoint_id = 5
  namespace   = "apps"
  name        = "deployer"
}`,
		resource.TestCheckResourceAttr(ds, "automount_service_account_token", "true"),
		resource.TestCheckResourceAttr(ds, "image_pull_secrets.#", "1"),
		resource.TestCheckResourceAttr(ds, "image_pull_secrets.0", "registry-creds"),
		resource.TestCheckResourceAttr(ds, "id", "portainer-kubernetes-service-account-5-apps-deployer"),
	)
}

// TestKubernetesApplicationDataSource covers the application inspect, whose
// deep structures stay in the raw details.
func TestKubernetesApplicationDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	const ds = "data.portainer_kubernetes_application.a"
	k8sDSRun(t, mock, `
data "portainer_kubernetes_application" "a" {
  endpoint_id   = 5
  namespace     = "apps"
  name          = "web"
  resource_type = "Deployment"
}`,
		resource.TestCheckResourceAttr(ds, "running_pods", "2"),
		resource.TestCheckResourceAttr(ds, "total_pods", "3"),
		resource.TestCheckResourceAttr(ds, "load_balancer_ip", "10.0.0.50"),
		// Pods are not flattened, so they have to survive in the raw details.
		resource.TestMatchResourceAttr(ds, "details", regexp.MustCompile(`web-0`)),
		k8sDSRequest(mock, "GET", "/kubernetes/5/namespaces/apps/applications/web", k8sDSQueryHas("resourceType=Deployment")),
	)
}

// TestKubernetesResourceCountsDataSource pins the seven calls this data
// source folds together, and that each lands on its own endpoint.
func TestKubernetesResourceCountsDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	const ds = "data.portainer_kubernetes_resource_counts.c"
	checks := make([]resource.TestCheckFunc, 0, 32)
	checks = append(checks,
		func(*terraform.State) error {
			calls := map[string]int{}
			for _, r := range mock.Requests() {
				if strings.HasPrefix(r.Path, "/kubernetes/5/") {
					calls[r.Path]++
				}
			}
			if len(calls) != 7 {
				return fmt.Errorf("expected seven count endpoints, got %v", calls)
			}
			for path, n := range calls {
				if !strings.HasSuffix(path, "/count") || n != calls["/kubernetes/5/applications/count"] {
					return fmt.Errorf("expected one call per count and read, got %v", calls)
				}
			}
			return nil
		},
	)
	for attribute, want := range map[string]string{
		"applications": "12", "namespaces": "4", "services": "20",
		"ingresses": "3", "config_maps": "17", "secrets": "9", "volumes": "6",
	} {
		checks = append(checks, resource.TestCheckResourceAttr(ds, attribute, want))
	}
	k8sDSRun(t, mock, `data "portainer_kubernetes_resource_counts" "c" { endpoint_id = 5 }`, checks...)
}

// TestKubernetesResourceCountsDataSource_NamesTheFailingCount keeps a single
// failing count from being reported as an opaque error.
func TestKubernetesResourceCountsDataSource_NamesTheFailingCount(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/5/applications/count", RespondJSON(http.StatusOK, 12))
	mock.On("GET", "/kubernetes/5/namespaces/count", RespondString(http.StatusForbidden, "application/json", `{"message":"forbidden"}`))
	k8sDSRunError(t, mock, `data "portainer_kubernetes_resource_counts" "c" { endpoint_id = 5 }`,
		`failed to count the namespaces of environment 5`)
}

// TestKubernetesCustomResourceDefinitionDataSources covers the two definition
// data sources.
func TestKubernetesCustomResourceDefinitionDataSources(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	const list, single = "data.portainer_kubernetes_custom_resource_definitions.l", "data.portainer_kubernetes_custom_resource_definition.s"
	k8sDSRun(t, mock, `
data "portainer_kubernetes_custom_resource_definitions" "l" { endpoint_id = 5 }
data "portainer_kubernetes_custom_resource_definition" "s" {
  endpoint_id = 5
  name        = "widgets.example.com"
}`,
		resource.TestCheckResourceAttr(list, "definitions.#", "2"),
		resource.TestCheckResourceAttr(list, "definitions.0.name", "widgets.example.com"),
		resource.TestCheckResourceAttr(list, "definitions.0.scope", "Namespaced"),
		resource.TestCheckResourceAttr(list, "definitions.0.release_name", "widgets"),
		resource.TestCheckResourceAttr(list, "definitions.0.release_version", "1.2.3"),
		resource.TestCheckResourceAttr(list, "id", "portainer-kubernetes-crds-5"),
		resource.TestCheckResourceAttr(single, "scope", "Namespaced"),
		// The name is an argument, so a response is never allowed to rewrite it.
		resource.TestCheckResourceAttr(single, "name", "widgets.example.com"),
		resource.TestCheckResourceAttr(single, "id", "portainer-kubernetes-crd-5-widgets.example.com"),
	)
}

// TestKubernetesCustomResourcesDataSource pins the required query
// parameter, without which Portainer does not know what to list.
func TestKubernetesCustomResourcesDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	const ds = "data.portainer_kubernetes_custom_resources.r"
	k8sDSRun(t, mock, `
data "portainer_kubernetes_custom_resources" "r" {
  endpoint_id = 5
  definition  = "widgets.example.com"
}`,
		resource.TestCheckResourceAttr(ds, "resources.#", "1"),
		resource.TestCheckResourceAttr(ds, "resources.0.namespace", "apps"),
		k8sDSRequest(mock, "GET", "/kubernetes/5/customresources", k8sDSQueryHas("definition=widgets.example.com")),
	)
}

// TestKubernetesCustomResourceDataSource_ScopeSelectsPath covers the two
// paths Portainer has: a namespaced resource and a cluster-scoped one, chosen
// by whether a namespace was given.
func TestKubernetesCustomResourceDataSource_ScopeSelectsPath(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	k8sDSRun(t, mock, `
data "portainer_kubernetes_custom_resource" "n" {
  endpoint_id = 5
  definition  = "widgets.example.com"
  name        = "prod"
  namespace   = "apps"
}
data "portainer_kubernetes_custom_resource" "c" {
  endpoint_id = 5
  definition  = "widgets.example.com"
  name        = "cluster-wide"
}`,
		resource.TestMatchResourceAttr("data.portainer_kubernetes_custom_resource.n", "manifest", regexp.MustCompile(`"kind":"Widget"`)),
		resource.TestCheckResourceAttr("data.portainer_kubernetes_custom_resource.c", "id",
			"portainer-kubernetes-custom-resource-5-widgets.example.com--cluster-wide"),
		k8sDSRequest(mock, "GET", "/kubernetes/5/customresources/cluster-wide", k8sDSQueryHas("definition=widgets.example.com")),
	)
}

// TestKubernetesGPUDataSource covers the GPU report, including a workload
// that could not be scheduled - which is the thing worth looking at in it.
func TestKubernetesGPUDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	const ds = "data.portainer_kubernetes_gpu.g"
	k8sDSRun(t, mock, `data "portainer_kubernetes_gpu" "g" { endpoint_id = 5 }`,
		resource.TestCheckResourceAttr(ds, "gpu_node_count", "2"),
		resource.TestCheckResourceAttr(ds, "degraded_count", "1"),
		resource.TestCheckResourceAttr(ds, "total_capacity.nvidia.com/gpu", "8"),
		resource.TestCheckResourceAttr(ds, "total_allocated.%", "0"),
		resource.TestCheckResourceAttr(ds, "nodes.#", "1"),
		resource.TestCheckResourceAttr(ds, "nodes.0.name", "gpu-1"),
		resource.TestCheckResourceAttr(ds, "workloads.#", "1"),
		resource.TestCheckResourceAttr(ds, "workloads.0.scheduling_issue", "insufficient nvidia.com/gpu"),
	)
}
