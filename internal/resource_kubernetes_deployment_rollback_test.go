package internal

import (
	"net/http"
	"strings"
	"testing"
)

// TestKubernetesDeploymentRollback_SendsRevision verifies the rollback request
// goes to the 2.45 rollback subresource with the requested revision.
func TestKubernetesDeploymentRollback_SendsRevision(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/kubernetes/2/namespaces/default/deployments/web/rollback", RespondJSON(http.StatusOK, map[string]interface{}{
		"spec": map[string]interface{}{"replicas": 4},
	}))

	r := resourceKubernetesDeploymentRollback()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 2)
	_ = d.Set("namespace", "default")
	_ = d.Set("deployment_name", "web")
	_ = d.Set("revision", 3)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if got := d.Get("rolled_back_to_replicas"); got != 4 {
		t.Errorf("rolled_back_to_replicas: expected 4, got %v", got)
	}
	// The ID carries a timestamp so a repeated rollback is a replacement.
	if !strings.HasPrefix(d.Id(), "2/default/web/rollback/") {
		t.Errorf("unexpected ID: %q", d.Id())
	}

	post := mock.FindRequest("POST", "/kubernetes/2/namespaces/default/deployments/web/rollback")
	if post == nil {
		t.Fatal("expected POST to the rollback endpoint")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if payload["revision"] != float64(3) {
		t.Errorf("payload.revision: expected 3, got %v", payload["revision"])
	}
}

// TestKubernetesDeploymentRollback_DefaultRevisionIsPrevious verifies the
// default revision of 0 is sent, which is how Portainer selects the previous
// revision (the kubectl rollout undo default).
func TestKubernetesDeploymentRollback_DefaultRevisionIsPrevious(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/kubernetes/2/namespaces/default/deployments/web/rollback", RespondJSON(http.StatusOK, map[string]interface{}{
		"spec": map[string]interface{}{"replicas": 1},
	}))

	r := resourceKubernetesDeploymentRollback()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 2)
	_ = d.Set("namespace", "default")
	_ = d.Set("deployment_name", "web")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	post := mock.FindRequest("POST", "/kubernetes/2/namespaces/default/deployments/web/rollback")
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if payload["revision"] != float64(0) {
		t.Errorf("payload.revision: expected 0 (previous revision), got %v", payload["revision"])
	}
}

// TestKubernetesDeploymentRollback_NoHistory verifies Portainer's 404 for a
// missing rollout target surfaces as an error.
func TestKubernetesDeploymentRollback_NoHistory(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/kubernetes/2/namespaces/default/deployments/web/rollback",
		RespondString(http.StatusNotFound, "application/json", `{"message":"Unable to find a revision to roll back to"}`))

	r := resourceKubernetesDeploymentRollback()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 2)
	_ = d.Set("namespace", "default")
	_ = d.Set("deployment_name", "web")
	_ = d.Set("revision", 99)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected Create to fail when there is no revision to roll back to")
	}
}
