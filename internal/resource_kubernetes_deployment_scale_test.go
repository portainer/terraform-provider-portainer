package internal

import (
	"net/http"
	"testing"
)

// TestKubernetesDeploymentScale_Create verifies the scale request goes to the
// 2.45 scale subresource and the status is stored from its response.
func TestKubernetesDeploymentScale_Create(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/kubernetes/3/namespaces/default/deployments/web/scale", RespondJSON(http.StatusOK, map[string]interface{}{
		"spec":   map[string]interface{}{"replicas": 3},
		"status": map[string]interface{}{"readyReplicas": 1, "availableReplicas": 1},
	}))

	r := resourceKubernetesDeploymentScale()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 3)
	_ = d.Set("namespace", "default")
	_ = d.Set("deployment_name", "web")
	_ = d.Set("replicas", 3)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if got := d.Id(); got != "3/default/web/scale" {
		t.Errorf("id: expected %q, got %q", "3/default/web/scale", got)
	}
	if got := d.Get("ready_replicas"); got != 1 {
		t.Errorf("ready_replicas: expected 1, got %v", got)
	}

	put := mock.FindRequest("PUT", "/kubernetes/3/namespaces/default/deployments/web/scale")
	if put == nil {
		t.Fatal("expected PUT to the scale endpoint")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if payload["replicas"] != float64(3) {
		t.Errorf("payload.replicas: expected 3, got %v", payload["replicas"])
	}
}

// TestKubernetesDeploymentScale_ReadDetectsDrift verifies a replica count
// changed outside Terraform comes back into state, so the next plan corrects it.
func TestKubernetesDeploymentScale_ReadDetectsDrift(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/kubernetes/3/namespaces/default/deployments/web", RespondJSON(http.StatusOK, map[string]interface{}{
		"spec":   map[string]interface{}{"replicas": 7},
		"status": map[string]interface{}{"readyReplicas": 7, "availableReplicas": 7},
	}))

	r := resourceKubernetesDeploymentScale()
	d := r.TestResourceData()
	d.SetId("3/default/web/scale")
	_ = d.Set("environment_id", 3)
	_ = d.Set("namespace", "default")
	_ = d.Set("deployment_name", "web")
	_ = d.Set("replicas", 3)

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("replicas"); got != 7 {
		t.Errorf("replicas: expected Read to adopt the cluster value 7, got %v", got)
	}
}

// TestKubernetesDeploymentScale_ReadMissingReplicasDefaultsToOne covers the
// Kubernetes pointer semantics of spec.replicas: absent means the default of 1,
// which is not the same as an explicit 0.
func TestKubernetesDeploymentScale_ReadMissingReplicasDefaultsToOne(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/kubernetes/3/namespaces/default/deployments/web", RespondJSON(http.StatusOK, map[string]interface{}{
		"spec":   map[string]interface{}{},
		"status": map[string]interface{}{},
	}))

	r := resourceKubernetesDeploymentScale()
	d := r.TestResourceData()
	d.SetId("3/default/web/scale")
	_ = d.Set("environment_id", 3)
	_ = d.Set("namespace", "default")
	_ = d.Set("deployment_name", "web")
	_ = d.Set("replicas", 5)

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("replicas"); got != 1 {
		t.Errorf("replicas: expected the Kubernetes default of 1, got %v", got)
	}
}

// TestKubernetesDeploymentScale_Read404ClearsID verifies a deployment deleted
// outside Terraform drops the resource from state rather than failing forever.
func TestKubernetesDeploymentScale_Read404ClearsID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/kubernetes/3/namespaces/default/deployments/gone",
		RespondString(http.StatusNotFound, "application/json", `{"message":"deployment not found"}`))

	r := resourceKubernetesDeploymentScale()
	d := r.TestResourceData()
	d.SetId("3/default/gone/scale")
	_ = d.Set("environment_id", 3)
	_ = d.Set("namespace", "default")
	_ = d.Set("deployment_name", "gone")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read should not fail on 404: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected the ID to be cleared, got %q", d.Id())
	}
}

// TestKubernetesDeploymentScale_ScaleToZeroAllowed verifies 0 is a valid
// replica count: it is how the workload is stopped without deleting it.
func TestKubernetesDeploymentScale_ScaleToZeroAllowed(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/kubernetes/3/namespaces/default/deployments/web/scale", RespondJSON(http.StatusOK, map[string]interface{}{
		"spec":   map[string]interface{}{"replicas": 0},
		"status": map[string]interface{}{},
	}))

	r := resourceKubernetesDeploymentScale()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 3)
	_ = d.Set("namespace", "default")
	_ = d.Set("deployment_name", "web")
	_ = d.Set("replicas", 0)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	put := mock.FindRequest("PUT", "/kubernetes/3/namespaces/default/deployments/web/scale")
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if payload["replicas"] != float64(0) {
		t.Errorf("payload.replicas: expected an explicit 0, got %v", payload["replicas"])
	}
}
