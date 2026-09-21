package internal

import (
	"net/http"
	"strings"
	"testing"
)

// TestDataSourceKubernetesCluster_HappyPath verifies the four cluster endpoints
// are folded into one data source, including the bare JSON boolean the RBAC
// endpoint answers with and the single-element list the dashboard returns.
func TestDataSourceKubernetesCluster_HappyPath(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/1/version", RespondJSON(http.StatusOK, map[string]interface{}{
		"gitVersion": "v1.31.4+k3s1", "major": "1", "minor": "31",
		"platform": "linux/amd64", "supportsPodRestart": true,
	}))
	mock.On("GET", "/kubernetes/1/rbac_enabled", RespondString(http.StatusOK, "application/json", "true"))
	mock.On("GET", "/kubernetes/1/dashboard", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"applicationsCount": 4, "namespacesCount": 6, "servicesCount": 9,
			"ingressesCount": 2, "configMapsCount": 11, "secretsCount": 7, "volumesCount": 3,
		},
	}))
	mock.On("GET", "/kubernetes/1/max_resource_limits", RespondJSON(http.StatusOK, map[string]interface{}{
		"CPU": 3800, "Memory": 7516192768,
	}))

	ds := dataSourceKubernetesCluster()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 1)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	for field, want := range map[string]interface{}{
		"version": "v1.31.4+k3s1", "major": "1", "minor": "31",
		"platform": "linux/amd64", "supports_pod_restart": true, "rbac_enabled": true,
		"applications_count": 4, "namespaces_count": 6, "services_count": 9,
		"ingresses_count": 2, "config_maps_count": 11, "secrets_count": 7, "volumes_count": 3,
		"max_cpu": 3800, "max_memory": 7516192768,
	} {
		if got := d.Get(field); got != want {
			t.Errorf("%s: expected %v, got %v", field, want, got)
		}
	}
}

// TestDataSourceKubernetesCluster_EmptyDashboard verifies an empty dashboard
// list does not panic — the counters simply stay at zero.
func TestDataSourceKubernetesCluster_EmptyDashboard(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/1/version", RespondJSON(http.StatusOK, map[string]interface{}{"gitVersion": "v1.31.4"}))
	mock.On("GET", "/kubernetes/1/rbac_enabled", RespondString(http.StatusOK, "application/json", "false"))
	mock.On("GET", "/kubernetes/1/dashboard", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("GET", "/kubernetes/1/max_resource_limits", RespondJSON(http.StatusOK, map[string]interface{}{}))

	ds := dataSourceKubernetesCluster()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 1)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("namespaces_count"); got != 0 {
		t.Errorf("namespaces_count: expected 0, got %v", got)
	}
	if got := d.Get("rbac_enabled"); got != false {
		t.Errorf("rbac_enabled: got %v", got)
	}
}

// TestDataSourceKubernetesNodes_MergesLimitsAndMetrics verifies the node list,
// Portainer's per-node limits and the metrics server are joined by node name.
func TestDataSourceKubernetesNodes_MergesLimitsAndMetrics(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/1/nodes", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"metadata": map[string]interface{}{
				"name": "worker-1", "labels": map[string]string{"node-role.kubernetes.io/worker": "true"},
			},
			"spec": map[string]interface{}{"unschedulable": true},
			"status": map[string]interface{}{
				"capacity":   map[string]string{"cpu": "4", "memory": "8Gi"},
				"conditions": []map[string]interface{}{{"type": "Ready", "status": "True"}},
				"nodeInfo":   map[string]interface{}{"kubeletVersion": "v1.31.4", "osImage": "Ubuntu 24.04"},
				"addresses":  []map[string]interface{}{{"type": "InternalIP", "address": "10.0.0.5"}},
			},
		},
	}))
	mock.On("GET", "/kubernetes/1/nodes_limits", RespondJSON(http.StatusOK, map[string]interface{}{
		"worker-1": map[string]interface{}{"CPU": 3500, "Memory": 6442450944},
	}))
	mock.On("GET", "/kubernetes/1/metrics/nodes", RespondJSON(http.StatusOK, map[string]interface{}{
		"items": []map[string]interface{}{
			{"metadata": map[string]interface{}{"name": "worker-1"}, "usage": map[string]string{"cpu": "250m", "memory": "1Gi"}},
		},
	}))

	ds := dataSourceKubernetesNodes()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 1)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	nodes := d.Get("nodes").([]interface{})
	if len(nodes) != 1 {
		t.Fatalf("nodes: expected 1 entry, got %d", len(nodes))
	}
	n := nodes[0].(map[string]interface{})
	for field, want := range map[string]interface{}{
		"name": "worker-1", "ready": true, "unschedulable": true,
		"kubelet_version": "v1.31.4", "internal_ip": "10.0.0.5",
		"capacity_cpu": "4", "available_cpu": 3500, "available_memory": 6442450944,
		"usage_cpu": "250m", "usage_memory": "1Gi",
	} {
		if got := n[field]; got != want {
			t.Errorf("nodes[0].%s: expected %v, got %v", field, want, got)
		}
	}
}

// TestDataSourceKubernetesNodes_WithoutMetricsServer verifies a cluster with no
// metrics-server still reads: usage is reported as unknown rather than failing.
func TestDataSourceKubernetesNodes_WithoutMetricsServer(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/1/nodes", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"metadata": map[string]interface{}{"name": "worker-1"},
			"status":   map[string]interface{}{"conditions": []map[string]interface{}{{"type": "Ready", "status": "False"}}},
		},
	}))
	mock.On("GET", "/kubernetes/1/nodes_limits", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/kubernetes/1/metrics/nodes",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"metrics API not available"}`))

	ds := dataSourceKubernetesNodes()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 1)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("a cluster without metrics-server must still read: %v", err)
	}
	n := d.Get("nodes").([]interface{})[0].(map[string]interface{})
	if n["usage_cpu"] != "" {
		t.Errorf("usage_cpu should be empty without metrics, got %v", n["usage_cpu"])
	}
	if n["ready"] != false {
		t.Errorf("ready: expected false, got %v", n["ready"])
	}
}

// TestDataSourceKubernetesEvents_CountsWarnings verifies the warning tally,
// which is what usually explains a workload that will not start.
func TestDataSourceKubernetesEvents_CountsWarnings(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/1/namespaces/prod/events", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"type": "Warning", "reason": "FailedScheduling", "message": "0/3 nodes are available",
			"count": 5, "involvedObject": map[string]interface{}{"kind": "Pod", "name": "web-1", "namespace": "prod", "uid": "u-1"},
			"firstTimestamp": "2026-09-01T10:00:00Z", "lastTimestamp": "2026-09-01T10:05:00Z",
		},
		{
			"type": "Normal", "reason": "Scheduled", "message": "assigned to worker-1", "count": 1,
			"involvedObject": map[string]interface{}{"kind": "Pod", "name": "web-2", "namespace": "prod"},
		},
	}))

	ds := dataSourceKubernetesEvents()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 1)
	_ = d.Set("namespace", "prod")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("warning_count"); got != 1 {
		t.Errorf("warning_count: expected 1, got %v", got)
	}
	e := d.Get("events").([]interface{})[0].(map[string]interface{})
	if e["reason"] != "FailedScheduling" || e["name"] != "web-1" || e["namespace"] != "prod" {
		t.Errorf("events[0] mismatch: %v", e)
	}
	if e["count"] != 5 {
		t.Errorf("events[0].count: got %v", e["count"])
	}
}

// TestDataSourceKubernetesDescribe_SendsQuery verifies kind, name and namespace
// reach the API as query parameters.
func TestDataSourceKubernetesDescribe_SendsQuery(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/1/describe", RespondJSON(http.StatusOK, map[string]interface{}{
		"describe": "Name: web-1\nStatus: Running\n",
	}))

	ds := dataSourceKubernetesDescribe()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 1)
	_ = d.Set("kind", "pod")
	_ = d.Set("name", "web-1")
	_ = d.Set("namespace", "prod")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if !strings.Contains(d.Get("describe").(string), "Status: Running") {
		t.Errorf("describe: got %q", d.Get("describe"))
	}
	get := mock.FindRequest("GET", "/kubernetes/1/describe")
	for _, want := range []string{"kind=pod", "name=web-1", "namespace=prod"} {
		if !strings.Contains(get.Query, want) {
			t.Errorf("query %q should contain %q", get.Query, want)
		}
	}
}

// TestDataSourceKubernetesPersistentVolumes_CountsReleased verifies the
// released tally, which is the storage a Retain policy has kept around.
func TestDataSourceKubernetesPersistentVolumes_CountsReleased(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/1/persistent_volumes", RespondJSON(http.StatusOK, []map[string]interface{}{
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
	}))

	ds := dataSourceKubernetesPersistentVolumes()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 1)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("released_count"); got != 1 {
		t.Errorf("released_count: expected 1, got %v", got)
	}
	pv := d.Get("persistent_volumes").([]interface{})[0].(map[string]interface{})
	if pv["claim_ref"] != "prod/data" || pv["capacity"] != "10Gi" {
		t.Errorf("persistent_volumes[0] mismatch: %v", pv)
	}
}

// TestDataSourceKubernetesConfig_YAMLBody verifies the kubeconfig is read as
// the YAML document it is, and that the environment filters reach the API.
func TestDataSourceKubernetesConfig_YAMLBody(t *testing.T) {
	mock := NewMockServer(t)
	kubeconfig := "apiVersion: v1\nkind: Config\nclusters: []\n"
	mock.On("GET", "/kubernetes/config", RespondString(http.StatusOK, "text/yaml", kubeconfig))

	ds := dataSourceKubernetesConfig()
	d := ds.TestResourceData()
	_ = d.Set("environment_ids", []interface{}{1, 2})
	_ = d.Set("exclude_environment_ids", []interface{}{3})

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("kubeconfig"); got != kubeconfig {
		t.Errorf("kubeconfig mismatch: %q", got)
	}
	if !dataSourceKubernetesConfig().Schema["kubeconfig"].Sensitive {
		t.Error("kubeconfig carries a bearer token and must be marked Sensitive")
	}

	get := mock.FindRequest("GET", "/kubernetes/config")
	for _, want := range []string{"ids=1", "ids=2", "excludeIds=3"} {
		if !strings.Contains(get.Query, want) {
			t.Errorf("query %q should contain %q", get.Query, want)
		}
	}
}

// TestDataSourceKubernetesCluster_DashboardBothShapes guards a decode failure
// an e2e run turned up. Portainer's API specification types the dashboard
// response as a list, but the server answers with a single object - and the
// mismatch took the whole data source down over a handful of counters.
func TestDataSourceKubernetesCluster_DashboardBothShapes(t *testing.T) {
	counters := map[string]interface{}{
		"applicationsCount": 3, "namespacesCount": 4, "servicesCount": 5,
		"ingressesCount": 1, "configMapsCount": 6, "secretsCount": 7, "volumesCount": 2,
	}

	for name, dashboard := range map[string]interface{}{
		"object the server actually returns": counters,
		"list the specification describes":   []interface{}{counters},
	} {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			mock.On("GET", "/kubernetes/4/version", RespondJSON(http.StatusOK, map[string]interface{}{
				"gitVersion": "v1.31.0", "major": "1", "minor": "31",
			}))
			mock.On("GET", "/kubernetes/4/rbac_enabled", RespondJSON(http.StatusOK, true))
			mock.On("GET", "/kubernetes/4/dashboard", RespondJSON(http.StatusOK, dashboard))
			mock.On("GET", "/kubernetes/4/max_resource_limits", RespondJSON(http.StatusOK, map[string]interface{}{
				"CPU": 4000, "Memory": 8192,
			}))

			ds := dataSourceKubernetesCluster()
			d := ds.TestResourceData()
			_ = d.Set("environment_id", 4)

			if err := rcRead(ds, d, mock.Client()); err != nil {
				t.Fatalf("Read failed: %v", err)
			}
			if got := d.Get("applications_count"); got != 3 {
				t.Errorf("applications_count: got %v", got)
			}
			if got := d.Get("volumes_count"); got != 2 {
				t.Errorf("volumes_count: got %v", got)
			}
		})
	}
}
