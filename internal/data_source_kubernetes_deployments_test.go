package internal

import (
	"net/http"
	"testing"
)

// TestDataSourceKubernetesDeployments_ClusterWide verifies an unset namespace
// uses the 2.45 cluster-wide route and that spec and status are both mapped.
func TestDataSourceKubernetesDeployments_ClusterWide(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/kubernetes/8/deployments", RespondJSON(http.StatusOK, map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"metadata": map[string]interface{}{
					"name": "web", "namespace": "default", "generation": 4,
					"labels":            map[string]string{"app": "web"},
					"creationTimestamp": "2026-08-01T09:00:00Z",
				},
				"spec": map[string]interface{}{
					"replicas": 3,
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"containers": []map[string]interface{}{
								{"image": "nginx:1.27"},
								{"image": "envoy:1.31"},
							},
						},
					},
				},
				"status": map[string]interface{}{
					"readyReplicas": 2, "availableReplicas": 2,
					"updatedReplicas": 3, "unavailableReplicas": 1,
					"observedGeneration": 3,
				},
			},
		},
	}))

	ds := dataSourceKubernetesDeployments()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 8)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	deployments := d.Get("deployments").([]interface{})
	if len(deployments) != 1 {
		t.Fatalf("deployments: expected 1 entry, got %d", len(deployments))
	}
	dep := deployments[0].(map[string]interface{})

	for field, want := range map[string]interface{}{
		"name": "web", "namespace": "default",
		"replicas": 3, "ready_replicas": 2, "available_replicas": 2,
		"updated_replicas": 3, "unavailable_replicas": 1,
		"generation": 4, "observed_generation": 3,
		"creation_timestamp": "2026-08-01T09:00:00Z",
	} {
		if got := dep[field]; got != want {
			t.Errorf("%s: expected %v, got %v", field, want, got)
		}
	}

	images := dep["images"].([]interface{})
	if len(images) != 2 || images[0] != "nginx:1.27" {
		t.Errorf("images: got %v", images)
	}
}

// TestDataSourceKubernetesDeployments_MissingReplicasDefaultsToOne covers the
// Kubernetes pointer semantics of spec.replicas: absent means 1, not 0.
func TestDataSourceKubernetesDeployments_MissingReplicasDefaultsToOne(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/kubernetes/8/namespaces/default/deployments", RespondJSON(http.StatusOK, map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"metadata": map[string]interface{}{"name": "web", "namespace": "default"},
				"spec":     map[string]interface{}{},
				"status":   map[string]interface{}{},
			},
		},
	}))

	ds := dataSourceKubernetesDeployments()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 8)
	_ = d.Set("namespace", "default")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	dep := d.Get("deployments").([]interface{})[0].(map[string]interface{})
	if dep["replicas"] != 1 {
		t.Errorf("replicas: expected the Kubernetes default of 1, got %v", dep["replicas"])
	}
	if dep["labels"] == nil {
		t.Error("labels: expected an empty map rather than nil")
	}
}

// TestDataSourceKubernetesDeployments_Selectors verifies both selectors reach
// the API as query parameters.
func TestDataSourceKubernetesDeployments_Selectors(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/kubernetes/8/deployments", RespondJSON(http.StatusOK, map[string]interface{}{
		"items": []map[string]interface{}{},
	}))

	ds := dataSourceKubernetesDeployments()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 8)
	_ = d.Set("label_selector", "app=web")
	_ = d.Set("field_selector", "metadata.name=web")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	get := mock.FindRequest("GET", "/kubernetes/8/deployments")
	if got := get.Query; got != "fieldSelector=metadata.name%3Dweb&labelSelector=app%3Dweb" {
		t.Errorf("unexpected query string: %q", got)
	}
}
