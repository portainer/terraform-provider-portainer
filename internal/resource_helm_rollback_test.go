package internal

import (
	"net/http"
	"strings"
	"testing"
)

// TestHelmRollbackCreate_HappyPath verifies that the action POSTs to
// /endpoints/{id}/kubernetes/helm/{release}/rollback with the expected query
// parameters built from the schema.
func TestHelmRollbackCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/4/kubernetes/helm/my-release/rollback",
		RespondString(http.StatusOK, "application/json", `{}`))

	r := resourceHelmRollback()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 4)
	_ = d.Set("release_name", "my-release")
	_ = d.Set("namespace", "default")
	_ = d.Set("revision", 2)
	_ = d.Set("wait", true)
	_ = d.Set("force", true)
	_ = d.Set("timeout", 120)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() == "" {
		t.Error("expected non-empty ID after rollback")
	}
	if !strings.HasPrefix(d.Id(), "helm-rollback-4-my-release-") {
		t.Errorf("unexpected ID format: %q", d.Id())
	}

	req := mock.FindRequest("POST", "/endpoints/4/kubernetes/helm/my-release/rollback")
	if req == nil {
		t.Fatal("expected POST to rollback endpoint")
	}
	q := req.Query
	for _, kv := range []string{"namespace=default", "revision=2", "wait=true", "force=true", "timeout=120"} {
		if !strings.Contains(q, kv) {
			t.Errorf("query %q missing expected %q", q, kv)
		}
	}
}

// TestHelmRollbackCreate_DefaultsOnly verifies the minimal call (no optional
// query parameters beyond the recreate default).
func TestHelmRollbackCreate_DefaultsOnly(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/kubernetes/helm/rel/rollback",
		RespondString(http.StatusOK, "application/json", `{}`))

	r := resourceHelmRollback()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("release_name", "rel")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if mock.FindRequest("POST", "/endpoints/1/kubernetes/helm/rel/rollback") == nil {
		t.Error("expected POST to rollback endpoint with minimal inputs")
	}
	if d.Id() == "" {
		t.Error("expected non-empty ID after minimal rollback")
	}
}

// TestHelmRollbackCreate_HTTPError verifies error propagation.
func TestHelmRollbackCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/kubernetes/helm/rel/rollback", RespondString(
		http.StatusBadRequest, "application/json",
		`{"message":"no previous revision"}`,
	))

	r := resourceHelmRollback()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("release_name", "rel")

	err := rcCreate(r, d, mock.Client())
	if err == nil {
		t.Fatal("expected error on 400, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID, got %q", d.Id())
	}
}
