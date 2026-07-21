package internal

import (
	"net/http"
	"testing"
)

// =========================================================================
// Additional coverage (cov3) for resource_stack.go focused on error and
// branch paths not exercised by the other stack test files:
//   - resourcePortainerStackUpdate repository path: POST /stacks/{id}/git
//     failure, and PUT /stacks/{id}/git/redeploy failure.
//   - resourcePortainerStackUpdate repository path AutoUpdate branches
//     (stack_webhook=true and update_interval-only) that build the AutoUpdate
//     payload.
//   - createStackSwarmRepo and createStackK8sRepo non-200 error branches.
// =========================================================================

// TestStackCov3_UpdateRepository_GitSettingsError covers the branch in
// resourcePortainerStackUpdate where POST /stacks/{id}/git returns a non-200,
// so the git settings update fails and the error is surfaced before redeploy.
func TestStackCov3_UpdateRepository_GitSettingsError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/stacks/90/git", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"git settings boom"}`,
	))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	d.SetId("90")
	_ = d.Set("method", "repository")
	_ = d.Set("name", "gitapp")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("repository_url", "https://github.com/acme/app.git")

	if err := rcUpdate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when POST /stacks/{id}/git returns 500, got nil")
	}
	// The redeploy PUT must not be attempted once git settings failed.
	if mock.FindRequest("PUT", "/stacks/90/git/redeploy") != nil {
		t.Error("did not expect redeploy PUT after git settings update failed")
	}
}

// TestStackCov3_UpdateRepository_RedeployError covers the branch where the git
// settings POST succeeds (200) but PUT /stacks/{id}/git/redeploy returns a
// non-200, so the redeploy failure is surfaced.
func TestStackCov3_UpdateRepository_RedeployError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/stacks/91/git", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("PUT", "/stacks/91/git/redeploy", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"redeploy boom"}`,
	))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	d.SetId("91")
	_ = d.Set("method", "repository")
	_ = d.Set("name", "gitapp")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("repository_url", "https://github.com/acme/app.git")

	if err := rcUpdate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when redeploy PUT returns 500, got nil")
	}
	if mock.FindRequest("POST", "/stacks/91/git") == nil {
		t.Error("expected git settings POST /stacks/91/git to have been sent")
	}
}

// TestStackCov3_UpdateRepository_WithWebhook covers the repository-update
// AutoUpdate branch triggered by stack_webhook=true: the git payload carries an
// AutoUpdate object and the resource generates a webhook_id.
func TestStackCov3_UpdateRepository_WithWebhook(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/stacks/92/git", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("PUT", "/stacks/92/git/redeploy", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/stacks/92", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 92, "Name": "gitwh", "Status": 1, "Type": 2, "EndpointId": 1,
		"gitConfig": map[string]interface{}{
			"URL":           "https://github.com/acme/app.git",
			"ReferenceName": "refs/heads/main",
		},
	}))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	d.SetId("92")
	_ = d.Set("method", "repository")
	_ = d.Set("name", "gitwh")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("repository_url", "https://github.com/acme/app.git")
	_ = d.Set("stack_webhook", true)
	_ = d.Set("update_interval", "10m")

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	gitPost := mock.FindRequest("POST", "/stacks/92/git")
	if gitPost == nil {
		t.Fatal("expected POST /stacks/92/git to be sent")
	}
	var payload map[string]interface{}
	if err := gitPost.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode git payload: %v", err)
	}
	if payload["AutoUpdate"] == nil {
		t.Error("expected AutoUpdate object in git settings payload when stack_webhook=true")
	}
}

// TestStackCov3_UpdateRepository_IntervalOnly covers the else-if branch in the
// repository update that builds an AutoUpdate object from update_interval alone
// (stack_webhook=false).
func TestStackCov3_UpdateRepository_IntervalOnly(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/stacks/93/git", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("PUT", "/stacks/93/git/redeploy", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/stacks/93", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 93, "Name": "gitint", "Status": 1, "Type": 2, "EndpointId": 1,
		"gitConfig": map[string]interface{}{
			"URL":           "https://github.com/acme/app.git",
			"ReferenceName": "refs/heads/main",
		},
	}))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	d.SetId("93")
	_ = d.Set("method", "repository")
	_ = d.Set("name", "gitint")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("repository_url", "https://github.com/acme/app.git")
	_ = d.Set("update_interval", "15m")
	// stack_webhook left false -> the else-if update_interval branch runs.

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	gitPost := mock.FindRequest("POST", "/stacks/93/git")
	if gitPost == nil {
		t.Fatal("expected POST /stacks/93/git to be sent")
	}
	var payload map[string]interface{}
	if err := gitPost.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode git payload: %v", err)
	}
	if payload["AutoUpdate"] == nil {
		t.Error("expected AutoUpdate object in git settings payload when update_interval is set")
	}
}

// TestStackCov3_SwarmRepositoryCreate_Error covers the non-200 branch of
// createStackSwarmRepo: the create POST fails, so Create returns an error and
// the resource ID stays empty.
func TestStackCov3_SwarmRepositoryCreate_Error(t *testing.T) {
	mock := NewMockServer(t)
	mockEmptyStackList(mock)

	mock.On("POST", "/stacks/create/swarm/repository", RespondString(
		http.StatusBadRequest, "application/json", `{"message":"bad swarm repo"}`,
	))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("deployment_type", "swarm")
	_ = d.Set("method", "repository")
	_ = d.Set("name", "gitswarm")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("swarm_id", "swarm-x") // set so fetchSwarmID is skipped
	_ = d.Set("repository_url", "https://github.com/acme/swarm.git")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on swarm repository create 400, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after failed create, got %q", d.Id())
	}
}

// TestStackCov3_KubernetesRepositoryCreate_Error covers the non-200 branch of
// createStackK8sRepo: the create POST fails, so Create returns an error and the
// resource ID stays empty.
func TestStackCov3_KubernetesRepositoryCreate_Error(t *testing.T) {
	mock := NewMockServer(t)
	mockEmptyStackList(mock)

	mock.On("POST", "/stacks/create/kubernetes/repository", RespondString(
		http.StatusBadRequest, "application/json", `{"message":"bad k8s repo"}`,
	))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("deployment_type", "kubernetes")
	_ = d.Set("method", "repository")
	_ = d.Set("name", "k8sgit")
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("namespace", "default")
	_ = d.Set("repository_url", "https://github.com/acme/k8s.git")
	_ = d.Set("file_path_in_repository", "manifest.yml")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on kubernetes repository create 400, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after failed create, got %q", d.Id())
	}
}
