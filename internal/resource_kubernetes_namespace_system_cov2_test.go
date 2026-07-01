package internal

import (
	"net/http"
	"testing"
)

// TestKubernetesNamespaceSystemCov2_Read_NoOp verifies Read is a pure no-op
// (returns nil and touches no endpoint).
func TestKubernetesNamespaceSystemCov2_Read_NoOp(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/1/namespaces/kube-system",
		RespondJSON(http.StatusOK, map[string]interface{}{"IsSystem": false}))

	r := resourceKubernetesNamespaceSystem()
	d := r.TestResourceData()
	d.SetId("1:kube-system")
	_ = d.Set("environment_id", 1)
	_ = d.Set("namespace", "kube-system")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read should be a no-op, got error: %v", err)
	}
	if d.Id() != "1:kube-system" {
		t.Errorf("Read must not mutate ID, got %q", d.Id())
	}
}
