package internal

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// TestKubernetesDeleteObjectCreate_HappyPath verifies the action POSTs to
// /kubernetes/{envId}/{resourceType}/delete with the namespace→names map and
// sets a composite ID combining envID, resource type, and names.
func TestKubernetesDeleteObjectCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/kubernetes/3/services/delete", RespondString(http.StatusOK, "application/json", `{}`))

	r := resourceKubernetesDeleteObject()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 3)
	_ = d.Set("resource_type", "services")
	_ = d.Set("namespace", "default")
	_ = d.Set("names", []interface{}{"web", "api"})

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	wantID := "3:services:web,api"
	if d.Id() != wantID {
		t.Errorf("ID: got %q, want %q", d.Id(), wantID)
	}

	req := mock.FindRequest("POST", "/kubernetes/3/services/delete")
	if req == nil {
		t.Fatal("expected POST /kubernetes/3/services/delete")
	}
	var body map[string][]string
	if err := json.Unmarshal(req.Body, &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	got, ok := body["default"]
	if !ok {
		t.Fatalf("expected key 'default' in body, got %v", body)
	}
	if strings.Join(got, ",") != "web,api" {
		t.Errorf("names: got %v, want [web api]", got)
	}
}

// TestKubernetesDeleteObjectCreate_HTTPError verifies that a server error is
// propagated and the ID stays empty.
func TestKubernetesDeleteObjectCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/kubernetes/1/services/delete", RespondString(
		http.StatusBadRequest, "application/json",
		`{"message":"invalid request"}`,
	))

	r := resourceKubernetesDeleteObject()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 1)
	_ = d.Set("resource_type", "services")
	_ = d.Set("namespace", "default")
	_ = d.Set("names", []interface{}{"x"})

	err := r.Create(d, mock.Client())
	if err == nil {
		t.Fatal("expected error on 400, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}

// TestKubernetesDeleteObjectDelete_ClearsID verifies that Delete clears the
// resource ID (Read/Delete are local no-ops since the resource is action-like).
func TestKubernetesDeleteObjectDelete_ClearsID(t *testing.T) {
	r := resourceKubernetesDeleteObject()
	d := r.TestResourceData()
	d.SetId("3:services:web")

	if err := r.Delete(d, nil); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}
