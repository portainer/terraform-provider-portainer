package internal

import (
	"net/http"
	"testing"
)

// TestDataSourceKubernetesResourceQuotas_HappyPath verifies the namespaced
// route is used and that limits and usage arrive as Kubernetes quantity
// strings.
func TestDataSourceKubernetesResourceQuotas_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/kubernetes/2/namespaces/prod/resource_quotas", RespondJSON(http.StatusOK, map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"metadata": map[string]interface{}{
					"name": "compute", "namespace": "prod",
					"creationTimestamp": "2026-07-01T08:00:00Z",
				},
				"spec": map[string]interface{}{
					"hard":   map[string]string{"limits.cpu": "4", "limits.memory": "8Gi"},
					"scopes": []string{"NotTerminating"},
				},
				"status": map[string]interface{}{
					"hard": map[string]string{"limits.cpu": "4", "limits.memory": "8Gi"},
					"used": map[string]string{"limits.cpu": "500m", "limits.memory": "1Gi"},
				},
			},
		},
	}))

	ds := dataSourceKubernetesResourceQuotas()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 2)
	_ = d.Set("namespace", "prod")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	quotas := d.Get("resource_quotas").([]interface{})
	if len(quotas) != 1 {
		t.Fatalf("resource_quotas: expected 1 entry, got %d", len(quotas))
	}
	q := quotas[0].(map[string]interface{})
	if q["name"] != "compute" || q["namespace"] != "prod" {
		t.Errorf("quota identity mismatch: %v", q)
	}
	hard := q["hard"].(map[string]interface{})
	if hard["limits.cpu"] != "4" || hard["limits.memory"] != "8Gi" {
		t.Errorf("hard: got %v", hard)
	}
	used := q["used"].(map[string]interface{})
	if used["limits.cpu"] != "500m" {
		t.Errorf("used: expected the quantity string 500m, got %v", used["limits.cpu"])
	}
	if scopes := q["scopes"].([]interface{}); len(scopes) != 1 || scopes[0] != "NotTerminating" {
		t.Errorf("scopes: got %v", scopes)
	}
	if got := d.Id(); got != "2/prod/resource_quotas" {
		t.Errorf("id: got %q", got)
	}
}

// TestDataSourceKubernetesResourceQuotas_FallsBackToSpecHard verifies that
// before the quota controller has observed the object, the spec limit is used
// rather than reporting no limit at all.
func TestDataSourceKubernetesResourceQuotas_FallsBackToSpecHard(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/kubernetes/2/namespaces/prod/resource_quotas", RespondJSON(http.StatusOK, map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"metadata": map[string]interface{}{"name": "fresh", "namespace": "prod"},
				"spec":     map[string]interface{}{"hard": map[string]string{"pods": "10"}},
				"status":   map[string]interface{}{},
			},
		},
	}))

	ds := dataSourceKubernetesResourceQuotas()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 2)
	_ = d.Set("namespace", "prod")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	q := d.Get("resource_quotas").([]interface{})[0].(map[string]interface{})
	hard := q["hard"].(map[string]interface{})
	if hard["pods"] != "10" {
		t.Errorf("hard: expected the spec limit when status is not populated yet, got %v", hard)
	}
	if q["used"] == nil {
		t.Error("used: expected an empty map rather than nil")
	}
	if q["scopes"] == nil {
		t.Error("scopes: expected an empty list rather than nil")
	}
}
