package internal

import (
	"net/http"
	"strings"
	"testing"
)

// TestDataSourceKubernetesReplicaSets_RevisionFromAnnotation verifies the
// rollout revision is read from the deployment.kubernetes.io/revision
// annotation, which is what the rollback resource takes as its target.
func TestDataSourceKubernetesReplicaSets_RevisionFromAnnotation(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/kubernetes/9/namespaces/default/replicasets", RespondJSON(http.StatusOK, map[string]interface{}{
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
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"containers": []map[string]interface{}{{"image": "nginx:1.27"}},
						},
					},
				},
				"status": map[string]interface{}{
					"readyReplicas": 3, "availableReplicas": 3, "fullyLabeledReplicas": 3,
				},
			},
		},
	}))

	ds := dataSourceKubernetesReplicaSets()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 9)
	_ = d.Set("namespace", "default")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	rs := d.Get("replicasets").([]interface{})[0].(map[string]interface{})
	if rs["revision"] != 7 {
		t.Errorf("revision: expected 7 from the annotation, got %v", rs["revision"])
	}
	if rs["name"] != "web-5d9f" || rs["fully_labeled_replicas"] != 3 {
		t.Errorf("replica set mismatch: %v", rs)
	}
}

// TestDataSourceKubernetesReplicaSets_NoRevisionAnnotation verifies a replica
// set no deployment owns reports revision 0 instead of failing the read.
func TestDataSourceKubernetesReplicaSets_NoRevisionAnnotation(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/kubernetes/9/namespaces/default/replicasets", RespondJSON(http.StatusOK, map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"metadata": map[string]interface{}{"name": "standalone", "namespace": "default"},
				"spec":     map[string]interface{}{},
				"status":   map[string]interface{}{},
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
	}))

	ds := dataSourceKubernetesReplicaSets()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 9)
	_ = d.Set("namespace", "default")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read should tolerate a missing or unparsable revision: %v", err)
	}

	items := d.Get("replicasets").([]interface{})
	for i, raw := range items {
		if got := raw.(map[string]interface{})["revision"]; got != 0 {
			t.Errorf("replicasets[%d].revision: expected 0, got %v", i, got)
		}
	}
	// spec.replicas absent means the Kubernetes default of 1.
	if got := items[0].(map[string]interface{})["replicas"]; got != 1 {
		t.Errorf("replicas: expected the Kubernetes default of 1, got %v", got)
	}
}

// TestDataSourceKubernetesReplicaSets_DeploymentFilter verifies the deployment
// filter is passed through as its own query parameter alongside the selectors.
func TestDataSourceKubernetesReplicaSets_DeploymentFilter(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/kubernetes/9/namespaces/default/replicasets", RespondJSON(http.StatusOK, map[string]interface{}{
		"items": []map[string]interface{}{},
	}))

	ds := dataSourceKubernetesReplicaSets()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 9)
	_ = d.Set("namespace", "default")
	_ = d.Set("deployment", "web")
	_ = d.Set("label_selector", "app=web")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	get := mock.FindRequest("GET", "/kubernetes/9/namespaces/default/replicasets")
	for _, want := range []string{"deployment=web", "labelSelector=app%3Dweb"} {
		if !strings.Contains(get.Query, want) {
			t.Errorf("query %q should contain %q", get.Query, want)
		}
	}
}
