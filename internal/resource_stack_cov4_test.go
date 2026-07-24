package internal

import (
	"net/http"
	"testing"
)

// =========================================================================
// Additional coverage (cov4) for resource_stack.go focused on happy-path CRUD
// branches not exercised by the other stack test files:
//
//   - Create non-repository finalize webhook block (stack_webhook=true): the
//     webhookToken generation + payload["webhook"] + webhook_id/webhook_url
//     setFields in resourcePortainerStackCreate.
//   - createStackStandaloneRepo autoUpdate-with-webhook branch + filesystem_path
//     + additional_files + registries optional payload fields.
//   - createStackSwarmRepo autoUpdate else-branch (update_interval only, no
//     webhook) + filesystem_path.
//   - createStackSwarmRepo prune=true post-create redeploy branch (invokes the
//     repository Update git/redeploy path from within Create).
//   - createStackK8sRepo helm_chart_path branch (manifestFile cleared,
//     helmChartPath + helmValuesFiles) combined with the autoUpdate webhook
//     branch.
//   - Non-repository Update webhook block (stack_webhook=true, method=string):
//     the second PUT /stacks/{id} carrying the webhook token + webhook_id/url
//     setFields.
//
// Skipped (framework-limited): branches gated on d.HasChange (e.g. the
// start/stop block in Update) cannot be triggered by TestResourceData, and the
// write-only (wo) repository credential branches require GetRawConfigAt, which
// TestResourceData does not populate.
// =========================================================================

// TestStackCov4_StandaloneString_WithWebhook covers the non-repository Create
// finalize block when stack_webhook=true: a webhook token is generated, added
// to the finalize PUT payload, and webhook_id/webhook_url are written to state.
func TestStackCov4_StandaloneString_WithWebhook(t *testing.T) {
	mock := NewMockServer(t)
	mockEmptyStackList(mock)

	mock.On("POST", "/stacks/create/standalone/string", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 210, "Name": "wh",
	}))
	mock.On("PUT", "/stacks/210", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 210, "Name": "wh"}))
	mock.On("GET", "/stacks/210", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 210, "Name": "wh", "Status": 1, "Type": 2, "EndpointId": 1,
	}))
	mock.On("GET", "/stacks/210/file", RespondJSON(http.StatusOK, map[string]interface{}{
		"StackFileContent": "version: '3'",
	}))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("deployment_type", "standalone")
	_ = d.Set("method", "string")
	_ = d.Set("name", "wh")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("stack_file_content", "version: '3'")
	_ = d.Set("stack_webhook", true)

	_ = d.Set("active", true)
	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "210" {
		t.Errorf("expected ID %q, got %q", "210", d.Id())
	}

	put := mock.FindRequest("PUT", "/stacks/210")
	if put == nil {
		t.Fatal("expected finalize PUT /stacks/210")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode finalize PUT body: %v", err)
	}
	if payload["webhook"] == nil || payload["webhook"] == "" {
		t.Errorf("expected finalize PUT payload to carry a webhook token, got %v", payload["webhook"])
	}
	// The Read that follows sets stack_webhook from the response (which carries
	// no webhook), so assert the token was recorded on the finalize PUT payload
	// rather than on final state.
}

// TestStackCov4_StandaloneRepo_WithWebhookAndInterval covers the
// createStackStandaloneRepo autoUpdate-with-webhook branch (stack_webhook=true),
// the filesystem_path optional field, and additional_files/registries payload
// population.
func TestStackCov4_StandaloneRepo_WithWebhookAndInterval(t *testing.T) {
	mock := NewMockServer(t)
	mockEmptyStackList(mock)

	mock.On("POST", "/stacks/create/standalone/repository", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 211, "Name": "gitwh",
	}))
	mock.On("GET", "/stacks/211", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 211, "Name": "gitwh", "Status": 1, "Type": 2, "EndpointId": 1,
		"gitConfig": map[string]interface{}{
			"URL":           "https://github.com/acme/app.git",
			"ReferenceName": "refs/heads/main",
		},
		// Carry a webhook so the trailing Read preserves webhook_id/url.
		"AutoUpdate": map[string]interface{}{"Webhook": "wh-211", "Interval": "5m"},
	}))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("deployment_type", "standalone")
	_ = d.Set("method", "repository")
	_ = d.Set("name", "gitwh")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("repository_url", "https://github.com/acme/app.git")
	_ = d.Set("stack_webhook", true)
	_ = d.Set("update_interval", "5m")
	_ = d.Set("filesystem_path", "/data/app")
	_ = d.Set("additional_files", []interface{}{"docker-compose.override.yml"})
	_ = d.Set("registries", []interface{}{1, 2})

	_ = d.Set("active", true)
	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "211" {
		t.Errorf("expected ID %q, got %q", "211", d.Id())
	}

	post := mock.FindRequest("POST", "/stacks/create/standalone/repository")
	if post == nil {
		t.Fatal("expected POST /stacks/create/standalone/repository")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode create body: %v", err)
	}
	if payload["autoUpdate"] == nil {
		t.Error("expected autoUpdate object when stack_webhook=true")
	}
	if got := payload["filesystemPath"]; got != "/data/app" {
		t.Errorf("payload.filesystemPath: expected /data/app, got %v", got)
	}
	files, ok := payload["additionalFiles"].([]interface{})
	if !ok || len(files) != 1 || files[0] != "docker-compose.override.yml" {
		t.Errorf("payload.additionalFiles: expected one override file, got %v", payload["additionalFiles"])
	}
	// stack_webhook=true generates a webhook_id and webhook_url in state.
	if got := d.Get("webhook_id"); got == "" {
		t.Error("expected webhook_id to be generated for repository webhook")
	}
}

// TestStackCov4_SwarmRepo_IntervalOnly covers the createStackSwarmRepo
// autoUpdate else-branch: update_interval is set but stack_webhook is false, so
// the autoUpdate object is built with an empty webhook id (no webhook_id/url
// side effects). Also exercises the filesystem_path optional field.
func TestStackCov4_SwarmRepo_IntervalOnly(t *testing.T) {
	mock := NewMockServer(t)
	mockEmptyStackList(mock)

	mock.On("POST", "/stacks/create/swarm/repository", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 212, "Name": "gitswarmint",
	}))
	mock.On("GET", "/stacks/212", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 212, "Name": "gitswarmint", "Status": 1, "Type": 1, "EndpointId": 1,
	}))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("deployment_type", "swarm")
	_ = d.Set("method", "repository")
	_ = d.Set("name", "gitswarmint")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("swarm_id", "swarm-int")
	_ = d.Set("repository_url", "https://github.com/acme/swarm.git")
	_ = d.Set("update_interval", "30m")
	_ = d.Set("filesystem_path", "/srv/stack")
	// stack_webhook intentionally left false -> autoUpdate built with empty webhook.

	_ = d.Set("active", true)
	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	post := mock.FindRequest("POST", "/stacks/create/swarm/repository")
	if post == nil {
		t.Fatal("expected POST /stacks/create/swarm/repository")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode create body: %v", err)
	}
	au, ok := payload["autoUpdate"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected autoUpdate object when update_interval is set, got %v", payload["autoUpdate"])
	}
	if got := au["interval"]; got != "30m" {
		t.Errorf("autoUpdate.interval: expected 30m, got %v", got)
	}
	if got := au["webhook"]; got != "" {
		t.Errorf("autoUpdate.webhook: expected empty (no stack_webhook), got %v", got)
	}
	if got := payload["filesystemPath"]; got != "/srv/stack" {
		t.Errorf("payload.filesystemPath: expected /srv/stack, got %v", got)
	}
	// No webhook_id should have been generated.
	if got := d.Get("webhook_id"); got != "" {
		t.Errorf("expected no webhook_id when stack_webhook is false, got %v", got)
	}
}

// TestStackCov4_SwarmRepo_PruneRedeploy covers the createStackSwarmRepo
// prune=true post-create branch: after the create POST sets the ID, Create
// invokes resourcePortainerStackUpdate (repository path) which issues the git
// settings POST + git/redeploy PUT.
func TestStackCov4_SwarmRepo_PruneRedeploy(t *testing.T) {
	mock := NewMockServer(t)
	mockEmptyStackList(mock)

	mock.On("POST", "/stacks/create/swarm/repository", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 213, "Name": "gitswarmprune",
	}))
	// prune=true triggers the immediate repository redeploy via Update.
	mock.On("POST", "/stacks/213/git", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("PUT", "/stacks/213/git/redeploy", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/stacks/213", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 213, "Name": "gitswarmprune", "Status": 1, "Type": 1, "EndpointId": 1,
		"gitConfig": map[string]interface{}{
			"URL":           "https://github.com/acme/swarm.git",
			"ReferenceName": "refs/heads/main",
		},
	}))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("deployment_type", "swarm")
	_ = d.Set("method", "repository")
	_ = d.Set("name", "gitswarmprune")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("swarm_id", "swarm-p")
	_ = d.Set("repository_url", "https://github.com/acme/swarm.git")
	_ = d.Set("prune", true)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "213" {
		t.Errorf("expected ID %q, got %q", "213", d.Id())
	}
	// The prune redeploy must have driven the repository Update git path.
	if mock.FindRequest("POST", "/stacks/213/git") == nil {
		t.Error("expected POST /stacks/213/git from prune redeploy")
	}
	if mock.FindRequest("PUT", "/stacks/213/git/redeploy") == nil {
		t.Error("expected PUT /stacks/213/git/redeploy from prune redeploy")
	}
}

// TestStackCov4_K8sRepo_HelmChartWithWebhook covers the createStackK8sRepo
// helm_chart_path branch (manifestFile cleared, helmChartPath + helmValuesFiles
// added) together with the autoUpdate-with-webhook branch.
func TestStackCov4_K8sRepo_HelmChartWithWebhook(t *testing.T) {
	mock := NewMockServer(t)
	mockEmptyStackList(mock)

	mock.On("POST", "/stacks/create/kubernetes/repository", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 214, "Name": "helmgit",
	}))
	mock.On("GET", "/stacks/214", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 214, "Name": "helmgit", "Status": 1, "Type": 3, "EndpointId": 2, "namespace": "default",
		"helmConfig": map[string]interface{}{
			"chartPath":   "charts/app",
			"valuesFiles": []string{"values-prod.yaml"},
		},
		// Carry a webhook so the trailing Read preserves webhook_id/url.
		"AutoUpdate": map[string]interface{}{"Webhook": "wh-214", "Interval": "1h"},
	}))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("deployment_type", "kubernetes")
	_ = d.Set("method", "repository")
	_ = d.Set("name", "helmgit")
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("namespace", "default")
	_ = d.Set("repository_url", "https://github.com/acme/helm.git")
	_ = d.Set("file_path_in_repository", "ignored-when-helm.yml")
	_ = d.Set("helm_chart_path", "charts/app")
	_ = d.Set("additional_helm_values_files", []interface{}{"values-prod.yaml"})
	_ = d.Set("stack_webhook", true)
	_ = d.Set("update_interval", "1h")

	_ = d.Set("active", true)
	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "214" {
		t.Errorf("expected ID %q, got %q", "214", d.Id())
	}

	post := mock.FindRequest("POST", "/stacks/create/kubernetes/repository")
	if post == nil {
		t.Fatal("expected POST /stacks/create/kubernetes/repository")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode create body: %v", err)
	}
	if got := payload["helmChartPath"]; got != "charts/app" {
		t.Errorf("payload.helmChartPath: expected charts/app, got %v", got)
	}
	// helm deployment clears manifestFile.
	if got := payload["manifestFile"]; got != "" {
		t.Errorf("payload.manifestFile: expected empty when helm_chart_path set, got %v", got)
	}
	hv, ok := payload["helmValuesFiles"].([]interface{})
	if !ok || len(hv) != 1 || hv[0] != "values-prod.yaml" {
		t.Errorf("payload.helmValuesFiles: expected [values-prod.yaml], got %v", payload["helmValuesFiles"])
	}
	if payload["autoUpdate"] == nil {
		t.Error("expected autoUpdate object when stack_webhook=true")
	}
	if got := d.Get("webhook_id"); got == "" {
		t.Error("expected webhook_id to be generated for helm repository webhook")
	}
}

// TestStackCov4_UpdateNonRepo_WithWebhook covers the non-repository Update
// webhook block (stack_webhook=true, method=string): after the standard content
// PUT, a second PUT /stacks/{id} carries the webhook token and webhook_id/url
// are written to state.
func TestStackCov4_UpdateNonRepo_WithWebhook(t *testing.T) {
	mock := NewMockServer(t)

	// The same path serves both the standard content PUT and the webhook PUT.
	mock.On("PUT", "/stacks/215", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 215, "Name": "app"}))
	// Carry a webhook token in the response so the trailing Read preserves
	// webhook_id/webhook_url (Read clears them when absent).
	mock.On("GET", "/stacks/215", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 215, "Name": "app", "Status": 1, "Type": 2, "EndpointId": 1,
		"webhook": "fixed-wh",
	}))
	mock.On("GET", "/stacks/215/file", RespondJSON(http.StatusOK, map[string]interface{}{
		"StackFileContent": "version: '3'",
	}))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	d.SetId("215")
	_ = d.Set("method", "string")
	_ = d.Set("name", "app")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("stack_file_content", "version: '3'")
	_ = d.Set("stack_webhook", true)

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// At least one PUT with a webhook token must have been sent.
	var sawWebhookPut bool
	for _, req := range mock.Requests() {
		if req.Method == http.MethodPut && req.Path == "/stacks/215" {
			var payload map[string]interface{}
			if err := req.DecodeJSON(&payload); err == nil {
				if wh, ok := payload["webhook"]; ok && wh != "" && wh != nil {
					sawWebhookPut = true
				}
			}
		}
	}
	if !sawWebhookPut {
		t.Error("expected a PUT /stacks/215 carrying a webhook token in the update webhook block")
	}
	if got := d.Get("webhook_id"); got == "" {
		t.Error("expected webhook_id to be set after the non-repository webhook update")
	}
	if got := d.Get("webhook_url"); got == "" {
		t.Error("expected webhook_url to be set after the non-repository webhook update")
	}
}
