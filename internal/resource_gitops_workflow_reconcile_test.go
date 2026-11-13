package internal

import (
	"net/http"
	"strings"
	"testing"
)

// TestGitopsWorkflowReconcile_PostsAndRecordsRun: the action is one POST with
// no body, and each run gets an ID of its own so a trigger change replaces it.
func TestGitopsWorkflowReconcile_PostsAndRecordsRun(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/gitops/workflows/4/reconcile", RespondString(http.StatusNoContent, "application/json", ""))

	r := resourceGitopsWorkflowReconcile()
	d := r.TestResourceData()
	_ = d.Set("workflow_id", 4)
	_ = d.Set("triggers", map[string]interface{}{"revision": "9f2c"})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if mock.FindRequest("POST", "/gitops/workflows/4/reconcile") == nil {
		t.Fatal("expected POST /gitops/workflows/4/reconcile")
	}
	if !strings.HasPrefix(d.Id(), "4-reconcile-") {
		t.Errorf("id: got %q", d.Id())
	}
}

// TestGitopsWorkflowReconcile_OlderServerNamesRequirement: Portainer 2.45 has
// no reconcile endpoint, and its 404 has to say why rather than "not found".
func TestGitopsWorkflowReconcile_OlderServerNamesRequirement(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/gitops/workflows/4/reconcile", RespondString(http.StatusNotFound, "text/plain", "404 page not found"))
	mock.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{
		"ServerVersion": "2.45.3", "ServerEdition": "BE",
	}))

	r := resourceGitopsWorkflowReconcile()
	d := r.TestResourceData()
	_ = d.Set("workflow_id", 4)

	err := rcCreate(r, d, mock.Client())
	if err == nil || !strings.Contains(err.Error(), "requires Portainer Business Edition 3.0 or later") {
		t.Fatalf("expected the requirement to be named, got %v", err)
	}
	if d.Id() != "" {
		t.Error("a failed reconcile must not be recorded")
	}
}
