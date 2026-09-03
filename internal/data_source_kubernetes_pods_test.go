package internal

import (
	"net/http"
	"testing"
)

// TestDataSourceKubernetesPods_ClusterWide verifies an unset namespace uses the
// 2.45 cluster-wide route and that readiness is aggregated from the container
// statuses.
func TestDataSourceKubernetesPods_ClusterWide(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/kubernetes/5/pods", RespondJSON(http.StatusOK, map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"metadata": map[string]interface{}{"name": "web-1", "namespace": "default", "labels": map[string]string{"app": "web"}},
				"spec": map[string]interface{}{
					"nodeName":           "worker-1",
					"serviceAccountName": "default",
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
		},
	}))

	ds := dataSourceKubernetesPods()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 5)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	pods := d.Get("pods").([]interface{})
	if len(pods) != 1 {
		t.Fatalf("pods: expected 1 entry, got %d", len(pods))
	}
	pod := pods[0].(map[string]interface{})
	if pod["name"] != "web-1" || pod["namespace"] != "default" {
		t.Errorf("pod identity mismatch: %v", pod)
	}
	if pod["node_name"] != "worker-1" || pod["phase"] != "Running" || pod["pod_ip"] != "10.1.0.5" {
		t.Errorf("pod status mismatch: %v", pod)
	}
	if pod["ready"] != true {
		t.Errorf("ready: expected true when every container is ready, got %v", pod["ready"])
	}
	if pod["restart_count"] != 3 {
		t.Errorf("restart_count: expected the sum across containers (3), got %v", pod["restart_count"])
	}
	if containers := pod["containers"].([]interface{}); len(containers) != 2 {
		t.Errorf("containers: expected 2 entries, got %d", len(containers))
	}
}

// TestDataSourceKubernetesPods_NamespacedWithSelectors verifies a configured
// namespace switches to the namespaced route and that both selectors reach the
// API as query parameters.
func TestDataSourceKubernetesPods_NamespacedWithSelectors(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/kubernetes/5/namespaces/prod/pods", RespondJSON(http.StatusOK, map[string]interface{}{
		"items": []map[string]interface{}{},
	}))

	ds := dataSourceKubernetesPods()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 5)
	_ = d.Set("namespace", "prod")
	_ = d.Set("label_selector", "app=web")
	_ = d.Set("field_selector", "status.phase=Running")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	get := mock.FindRequest("GET", "/kubernetes/5/namespaces/prod/pods")
	if get == nil {
		t.Fatal("expected the namespaced route to be used when namespace is set")
	}
	if got := get.Query; got != "fieldSelector=status.phase%3DRunning&labelSelector=app%3Dweb" {
		t.Errorf("unexpected query string: %q", got)
	}
}

// TestDataSourceKubernetesPods_NotReadyWhenStatusMissing verifies a pod whose
// kubelet has not reported container statuses yet is not claimed to be ready.
func TestDataSourceKubernetesPods_NotReadyWhenStatusMissing(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/kubernetes/5/pods", RespondJSON(http.StatusOK, map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"metadata": map[string]interface{}{"name": "pending-1", "namespace": "default"},
				"spec": map[string]interface{}{
					"containers": []map[string]interface{}{{"name": "web", "image": "nginx:1.27"}},
				},
				"status": map[string]interface{}{"phase": "Pending"},
			},
		},
	}))

	ds := dataSourceKubernetesPods()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 5)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	pod := d.Get("pods").([]interface{})[0].(map[string]interface{})
	if pod["ready"] != false {
		t.Errorf("ready: expected false while no container status is reported, got %v", pod["ready"])
	}
	if pod["labels"] == nil {
		t.Error("labels: expected an empty map rather than nil")
	}
}
