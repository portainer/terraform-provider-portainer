package internal

import (
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestKubernetesCRDDataSource_ByName fetches a single CRD by name and
// populates the crds list with one entry plus a deterministic ID.
func TestKubernetesCRDDataSource_ByName(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	const ds = "data.portainer_kubernetes_crd.c"
	k8sDSRun(t, mock, `
data "portainer_kubernetes_crd" "c" {
  environment_id = 5
  name           = "widgets.example.com"
}`,
		resource.TestCheckResourceAttr(ds, "id", "5/widgets.example.com"),
		resource.TestCheckResourceAttr(ds, "crds.#", "1"),
		resource.TestCheckResourceAttr(ds, "crds.0.name", "widgets.example.com"),
		resource.TestCheckResourceAttr(ds, "crds.0.group", "example.com"),
		resource.TestCheckResourceAttr(ds, "crds.0.scope", "Namespaced"),
		resource.TestCheckResourceAttr(ds, "crds.0.release_name", "widgets"),
	)
}

// TestKubernetesCRDDataSource_List fetches all CRDs for an environment and
// returns them as a list. The ID is a synthetic timestamp/envID string.
func TestKubernetesCRDDataSource_List(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	const ds = "data.portainer_kubernetes_crd.c"
	k8sDSRun(t, mock, `data "portainer_kubernetes_crd" "c" { environment_id = 5 }`,
		resource.TestMatchResourceAttr(ds, "id", regexp.MustCompile(`^\d+/5$`)),
		resource.TestCheckResourceAttr(ds, "crds.#", "2"),
		resource.TestCheckResourceAttr(ds, "crds.0.name", "widgets.example.com"),
		resource.TestCheckNoResourceAttr(ds, "name"),
	)
}

// TestKubernetesCRDDataSource_HTTPErrors surfaces HTTP 4xx/5xx on both paths.
func TestKubernetesCRDDataSource_HTTPErrors(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/5/customresourcedefinitions/missing",
		RespondString(http.StatusNotFound, "application/json", `{"message":"not found"}`))
	mock.On("GET", "/kubernetes/5/customresourcedefinitions",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	k8sDSRunError(t, mock, `
data "portainer_kubernetes_crd" "c" {
  environment_id = 5
  name           = "missing"
}`, `failed to get CRD`)
	k8sDSRunError(t, mock, `data "portainer_kubernetes_crd" "c" { environment_id = 5 }`, `failed to list CRDs`)
}

// TestKubernetesManifestDryRunDataSource_HappyPath verifies the data source
// POSTs the manifests to the dry-run endpoint and maps every field of the
// per-document result.
func TestKubernetesManifestDryRunDataSource_HappyPath(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/kubernetes/7/manifests/dry_run", RespondJSON(http.StatusOK, map[string]interface{}{
		"results": []map[string]interface{}{
			{"kind": "ConfigMap", "name": "app-config", "namespace": "default", "documentIndex": 0, "status": "pass", "message": ""},
			{"kind": "Service", "name": "app", "namespace": "default", "documentIndex": 1, "status": "pass", "message": ""},
		},
	}))
	const ds = "data.portainer_kubernetes_manifest_dry_run.d"
	k8sDSRun(t, mock, `
data "portainer_kubernetes_manifest_dry_run" "d" {
  environment_id = 7
  namespace      = "default"
  manifests      = ["kind: ConfigMap", "kind: Service"]
}`,
		resource.TestCheckResourceAttr(ds, "passed", "true"),
		resource.TestCheckResourceAttr(ds, "failed_count", "0"),
		resource.TestCheckResourceAttr(ds, "fail_on_invalid", "false"),
		resource.TestCheckResourceAttr(ds, "results.#", "2"),
		resource.TestCheckResourceAttr(ds, "results.0.kind", "ConfigMap"),
		resource.TestCheckResourceAttr(ds, "results.0.name", "app-config"),
		resource.TestCheckResourceAttr(ds, "results.0.status", "pass"),
		resource.TestCheckResourceAttr(ds, "results.0.document_index", "0"),
		k8sDSRequest(mock, "POST", "/kubernetes/7/manifests/dry_run", func(r *RecordedRequest) error {
			var payload map[string]interface{}
			if err := r.DecodeJSON(&payload); err != nil {
				return err
			}
			if payload["namespace"] != "default" {
				return fmt.Errorf("payload.namespace: got %v", payload["namespace"])
			}
			if manifests, ok := payload["manifests"].([]interface{}); !ok || len(manifests) != 2 {
				return fmt.Errorf("payload.manifests: expected 2 entries, got %v", payload["manifests"])
			}
			return nil
		}),
	)
}

// TestKubernetesManifestDryRunDataSource_InvalidReported verifies that an
// invalid document is reported rather than raised, so the configuration can
// decide what to do with it, and that the namespace is left out of the
// payload when it is not configured.
func TestKubernetesManifestDryRunDataSource_InvalidReported(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	const ds = "data.portainer_kubernetes_manifest_dry_run.d"
	k8sDSRun(t, mock, `
data "portainer_kubernetes_manifest_dry_run" "d" {
  environment_id = 7
  manifests      = ["kind: ConfigMap", ":::not yaml"]
}`,
		resource.TestCheckResourceAttr(ds, "passed", "false"),
		resource.TestCheckResourceAttr(ds, "failed_count", "1"),
		resource.TestMatchResourceAttr(ds, "id", regexp.MustCompile(`^\d+/7$`)),
		k8sDSRequest(mock, "POST", "/kubernetes/7/manifests/dry_run", func(r *RecordedRequest) error {
			var payload map[string]interface{}
			if err := r.DecodeJSON(&payload); err != nil {
				return err
			}
			if _, present := payload["namespace"]; present {
				return fmt.Errorf("payload should omit namespace when it is not configured, got %v", payload["namespace"])
			}
			return nil
		}),
	)
}

// TestKubernetesManifestDryRunDataSource_FailOnInvalid verifies the opt-in
// failure mode turns an invalid document into an error naming the document.
func TestKubernetesManifestDryRunDataSource_FailOnInvalid(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/kubernetes/1/manifests/dry_run", RespondJSON(http.StatusOK, map[string]interface{}{
		"results": []map[string]interface{}{
			{"kind": "Deployment", "name": "web", "namespace": "default", "documentIndex": 0, "status": "fail", "message": "spec.replicas: Invalid value"},
		},
	}))
	k8sDSRunError(t, mock, `
data "portainer_kubernetes_manifest_dry_run" "d" {
  environment_id  = 1
  manifests       = ["kind: Deployment"]
  fail_on_invalid = true
}`, `1 of 1 Kubernetes manifest document\(s\) failed validation:\s+document 0 \(Deployment/web\): spec.replicas: Invalid value`)
}

// TestKubernetesManifestDryRunDataSource_RequiresAManifest keeps MinItems.
func TestKubernetesManifestDryRunDataSource_RequiresAManifest(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	k8sDSRunError(t, mock, `
data "portainer_kubernetes_manifest_dry_run" "d" {
  environment_id = 1
  manifests      = []
}`, `at least 1`)
}

// TestKubernetesIngressClassesDataSource verifies the PascalCase payload of
// K8sIngressClass is decoded and the cluster default is surfaced, and that a
// cluster without a default reports an empty string.
func TestKubernetesIngressClassesDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	const ds = "data.portainer_kubernetes_ingress_classes.c"
	k8sDSRun(t, mock, `data "portainer_kubernetes_ingress_classes" "c" { environment_id = 1 }`,
		resource.TestCheckResourceAttr(ds, "ingress_classes.#", "2"),
		resource.TestCheckResourceAttr(ds, "ingress_classes.0.name", "traefik"),
		resource.TestCheckResourceAttr(ds, "ingress_classes.0.controller", "traefik.io/ingress-controller"),
		resource.TestCheckResourceAttr(ds, "ingress_classes.0.annotations.%", "0"),
		resource.TestCheckResourceAttr(ds, "ingress_classes.1.is_default", "true"),
		resource.TestCheckResourceAttr(ds, "default_ingress_class", "nginx"),
		resource.TestCheckResourceAttr(ds, "id", "1/ingressclasses"),
	)

	noDefault := NewMockServer(t)
	noDefault.On("GET", "/kubernetes/1/ingressclasses", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Name": "traefik", "Controller": "traefik.io/ingress-controller", "IsDefault": false},
	}))
	k8sDSRun(t, noDefault, `data "portainer_kubernetes_ingress_classes" "c" { environment_id = 1 }`,
		resource.TestCheckResourceAttr(ds, "default_ingress_class", ""),
	)
}

// TestKubernetesGatewayClassesDataSource verifies the PascalCase payload, and
// that an older Portainer is told apart from a plain failure.
func TestKubernetesGatewayClassesDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	const ds = "data.portainer_kubernetes_gateway_classes.c"
	k8sDSRun(t, mock, `data "portainer_kubernetes_gateway_classes" "c" { environment_id = 1 }`,
		resource.TestCheckResourceAttr(ds, "gateway_classes.0.controller_name", "gateway.envoyproxy.io/gatewayclass-controller"),
		resource.TestCheckResourceAttr(ds, "gateway_classes.0.labels.a", "b"),
		resource.TestCheckResourceAttr(ds, "gateway_classes.0.annotations.%", "0"),
		resource.TestCheckResourceAttr(ds, "id", "1/gatewayclasses"),
	)

	old := NewMockServer(t)
	old.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{
		"ServerVersion": "2.45.3", "ServerEdition": "CE",
	}))
	k8sDSRunError(t, old, `data "portainer_kubernetes_gateway_classes" "c" { environment_id = 1 }`,
		`portainer_kubernetes_gateway_classes requires`)
}

// TestKubernetesScopedKubeconfigsDataSource covers both listings - every
// environment and one - and the requirement message of an older Portainer.
func TestKubernetesScopedKubeconfigsDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	const all, one = "data.portainer_kubernetes_scoped_kubeconfigs.all", "data.portainer_kubernetes_scoped_kubeconfigs.one"
	k8sDSRun(t, mock, `
data "portainer_kubernetes_scoped_kubeconfigs" "all" {}
data "portainer_kubernetes_scoped_kubeconfigs" "one" { environment_id = 2 }`,
		resource.TestCheckResourceAttr(all, "id", "portainer-scoped-kubeconfigs"),
		resource.TestCheckResourceAttr(all, "scoped_kubeconfigs.#", "2"),
		resource.TestCheckResourceAttr(all, "scoped_kubeconfigs.1.namespaces.#", "0"),
		resource.TestCheckResourceAttr(one, "id", "2/scoped_kubeconfigs"),
		resource.TestCheckResourceAttr(one, "scoped_kubeconfigs.0.username", "admin"),
		resource.TestCheckResourceAttr(one, "scoped_kubeconfigs.0.environment_id", "2"),
		resource.TestCheckResourceAttr(one, "scoped_kubeconfigs.0.namespaces.0", "team-a"),
		resource.TestCheckResourceAttr(one, "scoped_kubeconfigs.0.expires_at", "4600"),
	)

	old := NewMockServer(t)
	old.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{
		"ServerVersion": "2.45.3", "ServerEdition": "BE",
	}))
	k8sDSRunError(t, old, `data "portainer_kubernetes_scoped_kubeconfigs" "all" {}`, `3.0 or later`)
}

// TestKubernetesNamespaceAccessDataSource reads users and teams ordered by ID,
// numerically.
func TestKubernetesNamespaceAccessDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	registerK8sDSFixtures(mock)
	const ds = "data.portainer_kubernetes_namespace_access.a"
	k8sDSRun(t, mock, `
data "portainer_kubernetes_namespace_access" "a" {
  endpoint_id  = 3
  namespace_id = "team-a"
}`,
		resource.TestCheckResourceAttr(ds, "user_access.0.user_id", "2"),
		resource.TestCheckResourceAttr(ds, "user_access.0.role_id", "1"),
		resource.TestCheckResourceAttr(ds, "user_access.0.namespaces.#", "0"),
		resource.TestCheckResourceAttr(ds, "user_access.1.user_id", "12"),
		resource.TestCheckResourceAttr(ds, "user_access.1.role_id", "4"),
		resource.TestCheckResourceAttr(ds, "user_access.1.namespaces.0", "team-a"),
		resource.TestCheckResourceAttr(ds, "team_access.0.team_id", "7"),
		resource.TestCheckResourceAttr(ds, "team_access.0.role_id", "3"),
		resource.TestCheckResourceAttr(ds, "id", "3:team-a"),
	)
}

func TestKubernetesNamespaceAccessDataSource_Errors(t *testing.T) {
	t.Parallel()
	const config = `
data "portainer_kubernetes_namespace_access" "a" {
  endpoint_id  = 3
  namespace_id = "team-a"
}`
	old := NewMockServer(t)
	old.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{
		"ServerVersion": "2.45.3", "ServerEdition": "BE",
	}))
	k8sDSRunError(t, old, config, `portainer_kubernetes_namespace_access requires`)

	unavailable := NewMockServer(t)
	unavailable.On("GET", "/endpoints/3/namespaces/team-a/access", RespondString(http.StatusServiceUnavailable, "", "not yet available"))
	k8sDSRunError(t, unavailable, config, `503`)
}
