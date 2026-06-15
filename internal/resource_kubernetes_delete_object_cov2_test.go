package internal

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestKubernetesDeleteObjectCov2_Read_NoOp verifies the Read handler is a no-op.
func TestKubernetesDeleteObjectCov2_Read_NoOp(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesDeleteObject()
	d := r.TestResourceData()
	d.SetId("3:services:web")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read should be a no-op, got error: %v", err)
	}
	if d.Id() != "3:services:web" {
		t.Errorf("Read must not mutate ID, got %q", d.Id())
	}
}

// TestKubernetesDeleteObjectCov2_Create_ClusterScoped covers a cluster-scoped
// resource type (cluster_roles) which uses an empty namespace key in the body
// map; the handler still POSTs to /kubernetes/{env}/{type}/delete.
func TestKubernetesDeleteObjectCov2_Create_ClusterScoped(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/kubernetes/5/cluster_roles/delete", RespondString(http.StatusOK, "application/json", `{}`))

	r := resourceKubernetesDeleteObject()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 5)
	_ = d.Set("resource_type", "cluster_roles")
	_ = d.Set("namespace", "")
	_ = d.Set("names", []interface{}{"admin"})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "5:cluster_roles:admin" {
		t.Errorf("unexpected ID %q", d.Id())
	}

	req := mock.FindRequest("POST", "/kubernetes/5/cluster_roles/delete")
	if req == nil {
		t.Fatal("expected POST recorded")
	}
	var body map[string][]string
	if err := json.Unmarshal(req.Body, &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got, ok := body[""]; !ok || len(got) != 1 || got[0] != "admin" {
		t.Errorf("expected body[\"\"]=[admin], got %v", body)
	}
}
