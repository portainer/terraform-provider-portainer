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

// k8sDSRun applies config (data sources only) against mock with this build
// and runs checks on the result.
func k8sDSRun(t *testing.T, mock *MockServer, config string, checks ...resource.TestCheckFunc) {
	t.Helper()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + config,
			Check:  resource.ComposeAggregateTestCheckFunc(checks...),
		}},
	})
}

// k8sDSRunError expects reading config to fail with an error matching re.
func k8sDSRunError(t *testing.T, mock *MockServer, config string, re string) {
	t.Helper()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + config,
			ExpectError: regexp.MustCompile(re),
		}},
	})
}

// k8sDSRequest checks the first request to method+path, which must exist.
func k8sDSRequest(mock *MockServer, method, path string, check func(r *RecordedRequest) error) resource.TestCheckFunc {
	return func(*terraform.State) error {
		r := mock.FindRequest(method, path)
		if r == nil {
			return fmt.Errorf("no %s %s request was made", method, path)
		}
		if check == nil {
			return nil
		}
		return check(r)
	}
}

// k8sDSQueryHas requires every want in the request's raw query string.
func k8sDSQueryHas(want ...string) func(r *RecordedRequest) error {
	return func(r *RecordedRequest) error {
		for _, w := range want {
			if !strings.Contains(r.Query, w) {
				return fmt.Errorf("query %q should contain %q", r.Query, w)
			}
		}
		return nil
	}
}

// k8sDSQueryIs requires the exact raw query string.
func k8sDSQueryIs(want string) func(r *RecordedRequest) error {
	return func(r *RecordedRequest) error {
		if r.Query != want {
			return fmt.Errorf("query: got %q, want %q", r.Query, want)
		}
		return nil
	}
}

// Canned Portainer responses for the read-only Kubernetes data sources,
// shared by their unit tests and by the v3.0.0 parity test. They include the
// shapes that matter for state: missing maps and lists, empty dashboards,
// unset optional arguments.

func k8sDSStorageClassJSON() map[string]interface{} {
	return map[string]interface{}{
		"name": "fast-ssd", "provisioner": "kubernetes.io/aws-ebs",
		"reclaimPolicy": "Delete", "isDefault": true, "allowVolumeExpansion": true,
		"mountOptions": []string{"noatime"},
		"parameters":   map[string]string{"type": "gp3"},
		"labels":       map[string]string{"tier": "fast"},
		"annotations":  map[string]string{"owner": "platform"},
		"creationDate": "2026-01-01",
	}
}

func k8sDSClaimJSON() map[string]interface{} {
	return map[string]interface{}{
		"id": "uid-1", "name": "data", "namespace": "apps", "phase": "Bound",
		"storageClass": "fast-ssd", "storageRequest": "10Gi", "storage": 10737418240,
		"volumeName": "pv-1", "volumeMode": "Filesystem",
		"accessModes": []string{"ReadWriteOnce"}, "humanReadableAccessModes": []string{"RWO"},
		"owningApplications": []string{"web"},
		"labels":             map[string]string{"app": "web"},
		"creationDate":       "2026-01-01",
	}
}

func k8sDSDefinitionJSON() map[string]interface{} {
	return map[string]interface{}{
		"name": "widgets.example.com", "group": "example.com", "scope": "Namespaced",
		"creationDate": "2026-01-01", "releaseName": "widgets", "releaseNamespace": "ops",
		"releaseVersion": "1.2.3",
	}
}

// registerK8sDSFixtures answers every endpoint the data sources read.
func registerK8sDSFixtures(mock *MockServer) {
	ok := func(method, path string, body interface{}) {
		mock.On(method, path, RespondJSON(http.StatusOK, body))
	}

	// portainer_kubernetes_crd
	ok("GET", "/kubernetes/5/customresourcedefinitions/widgets.example.com", k8sDSDefinitionJSON())
	ok("GET", "/kubernetes/5/customresourcedefinitions", []interface{}{
		k8sDSDefinitionJSON(),
		map[string]interface{}{"name": "gadgets.example.com", "group": "example.com", "scope": "Cluster", "creationDate": "2024-02-03T04:05:06Z"},
	})

	// portainer_kubernetes_manifest_dry_run
	ok("POST", "/kubernetes/7/manifests/dry_run", map[string]interface{}{
		"results": []map[string]interface{}{
			{"kind": "ConfigMap", "name": "app-config", "namespace": "default", "documentIndex": 0, "status": "pass", "message": ""},
			{"kind": "", "name": "", "namespace": "default", "documentIndex": 1, "status": "fail", "message": "error converting YAML to JSON"},
		},
	})

	// portainer_kubernetes_ingress_classes
	ok("GET", "/kubernetes/1/ingressclasses", []map[string]interface{}{
		{"Name": "traefik", "Controller": "traefik.io/ingress-controller", "IsDefault": false},
		{
			"Name": "nginx", "Controller": "k8s.io/ingress-nginx", "IsDefault": true,
			"Annotations": map[string]string{"ingressclass.kubernetes.io/is-default-class": "true"},
		},
	})

	// portainer_kubernetes_gateway_classes
	ok("GET", "/kubernetes/1/gatewayclasses", []map[string]interface{}{
		{
			"Name": "eg", "ControllerName": "gateway.envoyproxy.io/gatewayclass-controller", "UID": "u1",
			"Labels": map[string]string{"a": "b"},
		},
	})

	// portainer_kubernetes_scoped_kubeconfigs
	kubeconfigs := []map[string]interface{}{
		{
			"id": 5, "endpointId": 2, "userId": 1, "username": "admin", "active": true,
			"createdAt": 1000, "expiresAt": 4600, "description": "ci", "includeSecrets": false,
			"namespaces": []string{"team-a"}, "serviceAccount": "portainer-scoped-5",
		},
		{"id": 6, "endpointId": 2, "userId": 1, "username": "admin", "active": false},
	}
	ok("GET", "/kubernetes/scoped_kubeconfigs", kubeconfigs)
	ok("GET", "/kubernetes/2/scoped_kubeconfig", kubeconfigs[:1])

	// portainer_kubernetes_namespace_access
	ok("GET", "/endpoints/3/namespaces/team-a/access", map[string]interface{}{
		"UserAccessPolicies": map[string]interface{}{
			"12": map[string]interface{}{"RoleId": 4, "Namespaces": []string{"team-a"}},
			"2":  map[string]interface{}{"RoleId": 1},
		},
		"TeamAccessPolicies": map[string]interface{}{
			"7": map[string]interface{}{"RoleId": 3},
		},
	})

	// portainer_kubernetes_resource_quotas
	ok("GET", "/kubernetes/2/namespaces/prod/resource_quotas", map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"metadata": map[string]interface{}{"name": "compute", "namespace": "prod", "creationTimestamp": "2026-07-01T08:00:00Z"},
				"spec": map[string]interface{}{
					"hard":   map[string]string{"limits.cpu": "4", "limits.memory": "8Gi"},
					"scopes": []string{"NotTerminating"},
				},
				"status": map[string]interface{}{
					"hard": map[string]string{"limits.cpu": "4", "limits.memory": "8Gi"},
					"used": map[string]string{"limits.cpu": "500m", "limits.memory": "1Gi"},
				},
			},
			{
				"metadata": map[string]interface{}{"name": "fresh", "namespace": "prod"},
				"spec":     map[string]interface{}{"hard": map[string]string{"pods": "10"}},
				"status":   map[string]interface{}{},
			},
		},
	})

	// portainer_kubernetes_replicasets
	ok("GET", "/kubernetes/9/namespaces/default/replicasets", map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"metadata": map[string]interface{}{
					"name": "web-5d9f", "namespace": "default",
					"labels":            map[string]string{"app": "web"},
					"annotations":       map[string]string{"deployment.kubernetes.io/revision": "7"},
					"creationTimestamp": "2026-08-20T12:00:00Z",
				},
				"spec": map[string]interface{}{
					"replicas": 3,
					"template": map[string]interface{}{"spec": map[string]interface{}{
						"containers": []map[string]interface{}{{"image": "nginx:1.27"}},
					}},
				},
				"status": map[string]interface{}{"readyReplicas": 3, "availableReplicas": 3, "fullyLabeledReplicas": 3},
			},
			{
				"metadata": map[string]interface{}{
					"name": "garbled", "namespace": "default",
					"annotations": map[string]string{"deployment.kubernetes.io/revision": "not-a-number"},
				},
				"spec":   map[string]interface{}{},
				"status": map[string]interface{}{},
			},
		},
	})

	// portainer_kubernetes_pods
	pods := map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"metadata": map[string]interface{}{"name": "web-1", "namespace": "default", "labels": map[string]string{"app": "web"}},
				"spec": map[string]interface{}{
					"nodeName": "worker-1", "serviceAccountName": "default",
					"containers": []map[string]interface{}{
						{"name": "web", "image": "nginx:1.27"},
						{"name": "sidecar", "image": "envoy:1.31"},
					},
				},
				"status": map[string]interface{}{
					"phase": "Running", "podIP": "10.1.0.5", "hostIP": "192.168.1.10",
					"startTime": "2026-09-01T10:00:00Z",
					"containerStatuses": []map[string]interface{}{
						{"name": "web", "ready": true, "restartCount": 2},
						{"name": "sidecar", "ready": true, "restartCount": 1},
					},
				},
			},
			{
				"metadata": map[string]interface{}{"name": "pending-1", "namespace": "default"},
				"spec": map[string]interface{}{
					"containers": []map[string]interface{}{{"name": "web", "image": "nginx:1.27"}},
				},
				"status": map[string]interface{}{"phase": "Pending"},
			},
		},
	}
	ok("GET", "/kubernetes/5/pods", pods)
	ok("GET", "/kubernetes/5/namespaces/prod/pods", map[string]interface{}{"items": []interface{}{}})

	// portainer_kubernetes_pod_logs
	mock.On("GET", "/kubernetes/6/namespaces/default/pods/web-1/log",
		RespondString(http.StatusOK, "text/plain", "2026-09-01T10:00:00Z starting\n2026-09-01T10:00:01Z ready\n"))

	// portainer_kubernetes_deployments
	ok("GET", "/kubernetes/8/deployments", map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"metadata": map[string]interface{}{
					"name": "web", "namespace": "default", "generation": 4,
					"labels":            map[string]string{"app": "web"},
					"creationTimestamp": "2026-08-01T09:00:00Z",
				},
				"spec": map[string]interface{}{
					"replicas": 3,
					"template": map[string]interface{}{"spec": map[string]interface{}{
						"containers": []map[string]interface{}{{"image": "nginx:1.27"}, {"image": "envoy:1.31"}},
					}},
				},
				"status": map[string]interface{}{
					"readyReplicas": 2, "availableReplicas": 2, "updatedReplicas": 3,
					"unavailableReplicas": 1, "observedGeneration": 3,
				},
			},
		},
	})
	ok("GET", "/kubernetes/8/namespaces/default/deployments", map[string]interface{}{
		"items": []map[string]interface{}{
			{"metadata": map[string]interface{}{"name": "web", "namespace": "default"}, "spec": map[string]interface{}{}, "status": map[string]interface{}{}},
		},
	})

	// portainer_kubernetes_custom_resource(s)
	ok("GET", "/kubernetes/5/customresources", []map[string]interface{}{
		{"name": "prod", "namespace": "apps", "definitionName": "widgets.example.com", "uid": "abc", "creationDate": "2026-01-01"},
	})
	ok("GET", "/kubernetes/5/customresources/apps/prod", map[string]interface{}{"apiVersion": "example.com/v1", "kind": "Widget"})
	ok("GET", "/kubernetes/5/customresources/cluster-wide", map[string]interface{}{"apiVersion": "example.com/v1", "kind": "ClusterWidget"})

	// portainer_kubernetes_gpu
	ok("GET", "/kubernetes/5/gpu", map[string]interface{}{
		"Summary": map[string]interface{}{
			"GPUNodeCount": 2, "DegradedCount": 1, "OperatorDetected": true, "DevicePluginDetected": true,
			"TotalCapacity":    map[string]int64{"nvidia.com/gpu": 8},
			"TotalAllocatable": map[string]int64{"nvidia.com/gpu": 8},
		},
		"Nodes": []map[string]interface{}{
			{
				"Name": "gpu-1", "Status": "Ready", "StatusReason": "",
				"Capacity":    map[string]int64{"nvidia.com/gpu": 4},
				"Allocatable": map[string]int64{"nvidia.com/gpu": 4},
				"Allocated":   map[string]int64{"nvidia.com/gpu": 3},
			},
		},
		"Workloads": []map[string]interface{}{
			{
				"PodName": "trainer-0", "Namespace": "ml", "NodeName": "", "PodPhase": "Pending",
				"OwnerKind": "StatefulSet", "OwnerName": "trainer", "SchedulingIssue": "insufficient nvidia.com/gpu",
				"GPURequests": map[string]int64{"nvidia.com/gpu": 4},
			},
		},
	})

	// portainer_kubernetes_storage_class(es)
	bare := map[string]interface{}{"name": "bare", "provisioner": "rancher.io/local-path"}
	ok("GET", "/kubernetes/5/storage_classes", []interface{}{k8sDSStorageClassJSON(), bare})
	ok("GET", "/kubernetes/5/storage_classes/fast-ssd", k8sDSStorageClassJSON())
	ok("GET", "/kubernetes/5/storage_classes/bare", bare)

	// portainer_kubernetes_persistent_volume_claim(s)
	ok("GET", "/kubernetes/5/persistent_volume_claims", []interface{}{k8sDSClaimJSON(), map[string]interface{}{"name": "empty", "namespace": "apps"}})
	ok("GET", "/kubernetes/5/namespaces/apps/persistent_volume_claims", []interface{}{k8sDSClaimJSON()})
	ok("GET", "/kubernetes/5/namespaces/apps/persistent_volume_claims/data", k8sDSClaimJSON())

	// portainer_kubernetes_volume(s)
	ok("GET", "/kubernetes/5/volumes", map[string]interface{}{
		"items": []map[string]interface{}{{"persistentVolumeClaim": map[string]interface{}{"name": "data"}}},
	})
	ok("GET", "/kubernetes/5/namespaces/apps/volumes", map[string]interface{}{"items": []interface{}{}})
	ok("GET", "/kubernetes/5/volumes/apps/data", map[string]interface{}{
		"persistentVolume": map[string]interface{}{
			"name": "pv-1", "status": "Bound", "persistentVolumeReclaimPolicy": "Retain",
			"csi": map[string]interface{}{"driver": "ebs.csi.aws.com"},
		},
		"persistentVolumeClaim": map[string]interface{}{"name": "data", "phase": "Bound", "storageRequest": "10Gi"},
		"storageClass":          map[string]interface{}{"name": "fast-ssd", "provisioner": "ebs.csi.aws.com"},
	})

	// portainer_kubernetes_cron_jobs
	ok("GET", "/kubernetes/5/cron_jobs", []map[string]interface{}{
		{"Id": "1", "Name": "backup", "Namespace": "apps", "Schedule": "0 2 * * *", "Suspend": false, "IsSystem": false},
		{"Id": "2", "Name": "system-job", "Namespace": "kube-system", "Schedule": "* * * * *", "IsSystem": true},
	})

	// portainer_kubernetes_endpoints
	ok("GET", "/kubernetes/5/endpoints", []map[string]interface{}{
		{
			"uid": "uid-1", "name": "web", "namespace": "apps",
			"addresses": []string{"10.1.0.4"},
			"ports":     []map[string]interface{}{{"name": "http", "port": 8080, "protocol": "TCP"}},
		},
		{"uid": "uid-2", "name": "broken", "namespace": "apps", "addresses": []string{}},
		{"uid": "uid-3", "name": "bare", "namespace": "apps"},
	})

	// portainer_kubernetes_service_account
	ok("GET", "/kubernetes/5/namespaces/apps/service_accounts/deployer", map[string]interface{}{
		"uid": "uid-1", "isSystem": false, "automountServiceAccountToken": true,
		"imagePullSecrets": []string{"registry-creds"},
		"labels":           map[string]string{"app": "deployer"},
		"annotations":      map[string]string{"owner": "platform"},
		"creationDate":     "2026-01-01",
	})
	ok("GET", "/kubernetes/5/namespaces/apps/service_accounts/bare", map[string]interface{}{"uid": "uid-2"})

	// portainer_kubernetes_application
	ok("GET", "/kubernetes/5/namespaces/apps/applications/web", map[string]interface{}{
		"Uid": "uid-1", "Kind": "Deployment", "Image": "nginx:1.27",
		"Status": "Running", "RunningPodsCount": 2, "TotalPodsCount": 3,
		"ApplicationType": "Deployment", "ApplicationOwner": "platform",
		"DeploymentType": "Git", "StackId": "9", "StackName": "web",
		"ServiceName": "web", "ServiceType": "LoadBalancer",
		"LoadBalancerIPAddress": "10.0.0.50",
		"Labels":                map[string]string{"app": "web"},
		"CreationDate":          "2026-01-01",
		"Pods":                  []map[string]interface{}{{"Name": "web-0"}},
	})

	// portainer_kubernetes_resource_counts
	for path, count := range map[string]int{
		"applications": 12, "namespaces": 4, "services": 20, "ingresses": 3,
		"configmaps": 17, "secrets": 9, "volumes": 6,
	} {
		ok("GET", "/kubernetes/5/"+path+"/count", count)
	}

	// portainer_kubernetes_cluster: environment 1 has an empty dashboard
	// list, environment 4 the single object the server really returns.
	ok("GET", "/kubernetes/1/version", map[string]interface{}{"gitVersion": "v1.31.4"})
	mock.On("GET", "/kubernetes/1/rbac_enabled", RespondString(http.StatusOK, "application/json", "false"))
	ok("GET", "/kubernetes/1/dashboard", []map[string]interface{}{})
	ok("GET", "/kubernetes/1/max_resource_limits", map[string]interface{}{})
	ok("GET", "/kubernetes/4/version", map[string]interface{}{
		"gitVersion": "v1.31.4+k3s1", "major": "1", "minor": "31", "platform": "linux/amd64", "supportsPodRestart": true,
	})
	ok("GET", "/kubernetes/4/rbac_enabled", true)
	ok("GET", "/kubernetes/4/dashboard", map[string]interface{}{
		"applicationsCount": 3, "namespacesCount": 4, "servicesCount": 5,
		"ingressesCount": 1, "configMapsCount": 6, "secretsCount": 7, "volumesCount": 2,
	})
	ok("GET", "/kubernetes/4/max_resource_limits", map[string]interface{}{"CPU": 3800, "Memory": 7516192768})

	// portainer_kubernetes_nodes
	ok("GET", "/kubernetes/1/nodes", []map[string]interface{}{
		{
			"metadata": map[string]interface{}{"name": "worker-1", "labels": map[string]string{"node-role.kubernetes.io/worker": "true"}},
			"spec":     map[string]interface{}{"unschedulable": true},
			"status": map[string]interface{}{
				"capacity":   map[string]string{"cpu": "4", "memory": "8Gi"},
				"conditions": []map[string]interface{}{{"type": "Ready", "status": "True"}},
				"nodeInfo":   map[string]interface{}{"kubeletVersion": "v1.31.4", "osImage": "Ubuntu 24.04"},
				"addresses":  []map[string]interface{}{{"type": "InternalIP", "address": "10.0.0.5"}},
			},
		},
		{
			"metadata": map[string]interface{}{"name": "worker-2"},
			"status":   map[string]interface{}{"conditions": []map[string]interface{}{{"type": "Ready", "status": "False"}}},
		},
	})
	ok("GET", "/kubernetes/1/nodes_limits", map[string]interface{}{
		"worker-1": map[string]interface{}{"CPU": 3500, "Memory": 6442450944},
	})
	ok("GET", "/kubernetes/1/metrics/nodes", map[string]interface{}{
		"items": []map[string]interface{}{
			{"metadata": map[string]interface{}{"name": "worker-1"}, "usage": map[string]string{"cpu": "250m", "memory": "1Gi"}},
		},
	})

	// portainer_kubernetes_events
	events := []map[string]interface{}{
		{
			"type": "Warning", "reason": "FailedScheduling", "message": "0/3 nodes are available",
			"count": 5, "involvedObject": map[string]interface{}{"kind": "Pod", "name": "web-1", "namespace": "prod", "uid": "u-1"},
			"firstTimestamp": "2026-09-01T10:00:00Z", "lastTimestamp": "2026-09-01T10:05:00Z",
		},
		{
			"type": "Normal", "reason": "Scheduled", "message": "assigned to worker-1", "count": 1,
			"involvedObject": map[string]interface{}{"kind": "Pod", "name": "web-2", "namespace": "prod"},
		},
	}
	ok("GET", "/kubernetes/1/namespaces/prod/events", events)
	ok("GET", "/kubernetes/1/events", events[:1])

	// portainer_kubernetes_describe
	ok("GET", "/kubernetes/1/describe", map[string]interface{}{"describe": "Name: web-1\nStatus: Running\n"})

	// portainer_kubernetes_persistent_volumes
	ok("GET", "/kubernetes/1/persistent_volumes", []map[string]interface{}{
		{
			"metadata": map[string]interface{}{"name": "pv-1"},
			"spec": map[string]interface{}{
				"capacity": map[string]string{"storage": "10Gi"}, "accessModes": []string{"ReadWriteOnce"},
				"persistentVolumeReclaimPolicy": "Retain", "storageClassName": "fast",
				"claimRef": map[string]interface{}{"namespace": "prod", "name": "data"},
			},
			"status": map[string]interface{}{"phase": "Bound"},
		},
		{
			"metadata": map[string]interface{}{"name": "pv-2"},
			"spec":     map[string]interface{}{"persistentVolumeReclaimPolicy": "Retain"},
			"status":   map[string]interface{}{"phase": "Released"},
		},
	})

	// portainer_kubernetes_config
	mock.On("GET", "/kubernetes/config", RespondString(http.StatusOK, "text/yaml", "apiVersion: v1\nkind: Config\nclusters: []\n"))

	// portainer_kubernetes_pod_metrics / node_metrics / application_resources
	ok("GET", "/kubernetes/1/metrics/pods/namespace/prod", map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"metadata": map[string]interface{}{"name": "web-1"}, "timestamp": "2026-09-03T10:00:00Z", "window": "30s",
				"containers": []map[string]interface{}{{"name": "web", "usage": map[string]string{"cpu": "250m", "memory": "1Gi"}}},
			},
		},
	})
	ok("GET", "/kubernetes/1/metrics/pods/prod/web-1", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "web-1"}, "window": "30s",
		"containers": []map[string]interface{}{{"name": "web", "usage": map[string]string{"cpu": "300m", "memory": "2Gi"}}},
	})
	ok("GET", "/kubernetes/1/metrics/nodes/worker-1", map[string]interface{}{
		"timestamp": "2026-09-03T10:00:00Z", "window": "30s",
		"usage": map[string]string{"cpu": "1200m", "memory": "3Gi"},
	})
	ok("GET", "/kubernetes/1/metrics/applications_resources", map[string]interface{}{
		"CpuRequest": 2.5, "CpuLimit": 4.0, "MemoryRequest": 2147483648, "MemoryLimit": 4294967296,
	})
}

// k8sDSParityConfig reads every data source, once with its optional
// arguments set and once without where that changes what is stored.
const k8sDSParityConfig = `
data "portainer_kubernetes_crd" "one" {
  environment_id = 5
  name           = "widgets.example.com"
}
data "portainer_kubernetes_crd" "all" { environment_id = 5 }

data "portainer_kubernetes_manifest_dry_run" "default" {
  environment_id = 7
  manifests      = ["kind: ConfigMap", ":::not yaml"]
}
data "portainer_kubernetes_manifest_dry_run" "namespaced" {
  environment_id  = 7
  manifests       = ["kind: ConfigMap"]
  namespace       = "default"
  fail_on_invalid = false
}

data "portainer_kubernetes_ingress_classes" "c" { environment_id = 1 }
data "portainer_kubernetes_gateway_classes" "c" { environment_id = 1 }

data "portainer_kubernetes_scoped_kubeconfigs" "all" {}
data "portainer_kubernetes_scoped_kubeconfigs" "one" { environment_id = 2 }

data "portainer_kubernetes_namespace_access" "a" {
  endpoint_id  = 3
  namespace_id = "team-a"
}

data "portainer_kubernetes_resource_quotas" "q" {
  environment_id = 2
  namespace      = "prod"
}

data "portainer_kubernetes_replicasets" "plain" {
  environment_id = 9
  namespace      = "default"
}
data "portainer_kubernetes_replicasets" "filtered" {
  environment_id = 9
  namespace      = "default"
  deployment     = "web"
  label_selector = "app=web"
  field_selector = "metadata.name=web-5d9f"
}

data "portainer_kubernetes_pods" "all" { environment_id = 5 }
data "portainer_kubernetes_pods" "prod" {
  environment_id = 5
  namespace      = "prod"
  label_selector = "app=web"
  field_selector = "status.phase=Running"
}

data "portainer_kubernetes_pod_logs" "plain" {
  environment_id = 6
  namespace      = "default"
  pod_name       = "web-1"
}
data "portainer_kubernetes_pod_logs" "full" {
  environment_id = 6
  namespace      = "default"
  pod_name       = "web-1"
  container      = "web"
  tail_lines     = 200
  since_seconds  = 3600
  timestamps     = true
  previous       = true
}

data "portainer_kubernetes_deployments" "all" { environment_id = 8 }
data "portainer_kubernetes_deployments" "ns" {
  environment_id = 8
  namespace      = "default"
}

data "portainer_kubernetes_custom_resource_definitions" "d" { endpoint_id = 5 }
data "portainer_kubernetes_custom_resource_definition" "d" {
  endpoint_id = 5
  name        = "widgets.example.com"
}
data "portainer_kubernetes_custom_resources" "r" {
  endpoint_id = 5
  definition  = "widgets.example.com"
}
data "portainer_kubernetes_custom_resource" "namespaced" {
  endpoint_id = 5
  definition  = "widgets.example.com"
  name        = "prod"
  namespace   = "apps"
}
data "portainer_kubernetes_custom_resource" "cluster" {
  endpoint_id = 5
  definition  = "widgets.example.com"
  name        = "cluster-wide"
}

data "portainer_kubernetes_gpu" "g" { endpoint_id = 5 }

data "portainer_kubernetes_storage_classes" "s" { endpoint_id = 5 }
data "portainer_kubernetes_storage_class" "fast" {
  endpoint_id = 5
  name        = "fast-ssd"
}
data "portainer_kubernetes_storage_class" "bare" {
  endpoint_id = 5
  name        = "bare"
}

data "portainer_kubernetes_persistent_volume_claims" "all" { endpoint_id = 5 }
data "portainer_kubernetes_persistent_volume_claims" "apps" {
  endpoint_id = 5
  namespace   = "apps"
}
data "portainer_kubernetes_persistent_volume_claim" "c" {
  endpoint_id = 5
  namespace   = "apps"
  name        = "data"
}

data "portainer_kubernetes_volumes" "all" { endpoint_id = 5 }
data "portainer_kubernetes_volumes" "apps" {
  endpoint_id       = 5
  namespace         = "apps"
  with_applications = true
}
data "portainer_kubernetes_volume" "v" {
  endpoint_id = 5
  namespace   = "apps"
  volume      = "data"
}

data "portainer_kubernetes_cron_jobs" "default" { endpoint_id = 5 }
data "portainer_kubernetes_cron_jobs" "system" {
  endpoint_id    = 5
  include_system = true
}

data "portainer_kubernetes_endpoints" "e" { endpoint_id = 5 }

data "portainer_kubernetes_service_account" "full" {
  endpoint_id = 5
  namespace   = "apps"
  name        = "deployer"
}
data "portainer_kubernetes_service_account" "bare" {
  endpoint_id = 5
  namespace   = "apps"
  name        = "bare"
}

data "portainer_kubernetes_application" "plain" {
  endpoint_id = 5
  namespace   = "apps"
  name        = "web"
}
data "portainer_kubernetes_application" "typed" {
  endpoint_id   = 5
  namespace     = "apps"
  name          = "web"
  resource_type = "Deployment"
}

data "portainer_kubernetes_resource_counts" "c" { endpoint_id = 5 }

data "portainer_kubernetes_cluster" "empty" { environment_id = 1 }
data "portainer_kubernetes_cluster" "full" { environment_id = 4 }

data "portainer_kubernetes_nodes" "n" { environment_id = 1 }

data "portainer_kubernetes_events" "prod" {
  environment_id = 1
  namespace      = "prod"
}
data "portainer_kubernetes_events" "uid" {
  environment_id = 1
  resource_id    = "u-1"
}

data "portainer_kubernetes_describe" "pod" {
  environment_id = 1
  kind           = "pod"
  name           = "web-1"
  namespace      = "prod"
}
data "portainer_kubernetes_describe" "node" {
  environment_id = 1
  kind           = "node"
  name           = "worker-1"
}

data "portainer_kubernetes_persistent_volumes" "p" { environment_id = 1 }

data "portainer_kubernetes_config" "all" {}
data "portainer_kubernetes_config" "some" {
  environment_ids         = [1, 2]
  exclude_environment_ids = [3]
}

data "portainer_kubernetes_pod_metrics" "ns" {
  environment_id = 1
  namespace      = "prod"
}
data "portainer_kubernetes_pod_metrics" "one" {
  environment_id = 1
  namespace      = "prod"
  pod            = "web-1"
}

data "portainer_kubernetes_node_metrics" "n" {
  environment_id = 1
  node           = "worker-1"
}

data "portainer_kubernetes_application_resources" "r" { environment_id = 1 }
`
