package internal

import (
	"net/http"
	"strings"
	"testing"
)

// TestDataSourceGitopsSources_ListAndSummary verifies the list is mapped, the
// filters reach the API as query parameters, and the separate summary endpoint
// is folded into the same data source.
func TestDataSourceGitopsSources_ListAndSummary(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/gitops/sources", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"id": 1, "name": "infra", "url": "https://github.com/acme/infra.git",
			"type": "git", "status": "healthy", "interval": "5m",
			"lastSync": 1756900000, "usedBy": 3, "environments": 2,
		},
		{
			"id": 2, "name": "apps", "url": "https://github.com/acme/apps.git",
			"type": "git", "status": "error", "error": "authentication failed",
		},
	}))
	mock.On("GET", "/gitops/sources/summary", RespondJSON(http.StatusOK, map[string]interface{}{
		"healthy": 1, "syncing": 0, "error": 1, "paused": 0, "unknown": 0,
	}))

	ds := dataSourceGitopsSources()
	d := ds.TestResourceData()
	_ = d.Set("search", "acme")
	_ = d.Set("status", "healthy")
	_ = d.Set("limit", 50)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	sources := d.Get("sources").([]interface{})
	if len(sources) != 2 {
		t.Fatalf("sources: expected 2 entries, got %d", len(sources))
	}
	first := sources[0].(map[string]interface{})
	if first["name"] != "infra" || first["used_by"] != 3 || first["environments"] != 2 {
		t.Errorf("sources[0] mismatch: %v", first)
	}
	second := sources[1].(map[string]interface{})
	if second["status"] != "error" || second["status_error"] != "authentication failed" {
		t.Errorf("sources[1] should carry the sync error: %v", second)
	}

	summary := d.Get("summary").([]interface{})
	if len(summary) != 1 {
		t.Fatalf("summary: expected exactly one element, got %d", len(summary))
	}
	if s := summary[0].(map[string]interface{}); s["healthy"] != 1 || s["error"] != 1 {
		t.Errorf("summary mismatch: %v", s)
	}

	get := mock.FindRequest("GET", "/gitops/sources")
	for _, want := range []string{"search=acme", "status=healthy", "limit=50"} {
		if !strings.Contains(get.Query, want) {
			t.Errorf("query %q should contain %q", get.Query, want)
		}
	}
}

// TestDataSourceGitopsSource_WithWorkflows verifies the single-source data
// source also resolves the workflows using it — the reason a source cannot be
// deleted while it is in use.
func TestDataSourceGitopsSource_WithWorkflows(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/gitops/sources/12", RespondJSON(http.StatusOK, gitopsSourceDetailBody(12)))
	mock.On("GET", "/gitops/sources/12/workflows", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"id": 5, "name": "web", "type": "edgeStack", "platform": "kubernetes",
			"target":       map[string]interface{}{"namespace": "prod", "edgeGroupIds": []int{1, 2}},
			"creationDate": 1756800000, "lastSyncDate": 1756900000,
		},
	}))

	ds := dataSourceGitopsSource()
	d := ds.TestResourceData()
	_ = d.Set("source_id", 12)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Get("name"); got != "infra" {
		t.Errorf("name: got %v", got)
	}
	if got := d.Get("tls_skip_verify"); got != true {
		t.Errorf("tls_skip_verify: got %v", got)
	}
	if users := d.Get("user_accesses").([]interface{}); len(users) != 2 {
		t.Errorf("user_accesses: got %v", users)
	}

	workflows := d.Get("workflows").([]interface{})
	if len(workflows) != 1 {
		t.Fatalf("workflows: expected 1 entry, got %d", len(workflows))
	}
	w := workflows[0].(map[string]interface{})
	if w["name"] != "web" || w["platform"] != "kubernetes" || w["namespace"] != "prod" {
		t.Errorf("workflow mismatch: %v", w)
	}
	if groups := w["edge_group_ids"].([]interface{}); len(groups) != 2 {
		t.Errorf("edge_group_ids: got %v", groups)
	}
}

// TestDataSourceGitopsWorkflows_HealthAggregation verifies the three phase
// statuses are exposed separately and that `healthy` is only true when all
// three are healthy.
func TestDataSourceGitopsWorkflows_HealthAggregation(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/gitops/workflows", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"id": 1, "name": "all-good",
			"status": map[string]interface{}{
				"source":   map[string]interface{}{"status": "healthy"},
				"artifact": map[string]interface{}{"status": "healthy"},
				"target":   map[string]interface{}{"status": "healthy"},
			},
		},
		{
			"id": 2, "name": "target-broken",
			"status": map[string]interface{}{
				"source":   map[string]interface{}{"status": "healthy"},
				"artifact": map[string]interface{}{"status": "healthy"},
				"target":   map[string]interface{}{"status": "error", "error": "namespace not found"},
			},
		},
	}))
	mock.On("GET", "/gitops/workflows/summary", RespondJSON(http.StatusOK, map[string]interface{}{
		"healthy": 1, "error": 1,
	}))

	ds := dataSourceGitopsWorkflows()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_ids", []interface{}{3, 4})
	_ = d.Set("platform", "kubernetes")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	workflows := d.Get("workflows").([]interface{})
	if len(workflows) != 2 {
		t.Fatalf("workflows: expected 2 entries, got %d", len(workflows))
	}
	if got := workflows[0].(map[string]interface{})["healthy"]; got != true {
		t.Errorf("workflows[0].healthy: expected true, got %v", got)
	}
	broken := workflows[1].(map[string]interface{})
	if broken["healthy"] != false {
		t.Errorf("workflows[1].healthy: expected false when a phase errors, got %v", broken["healthy"])
	}
	if broken["target_error"] != "namespace not found" {
		t.Errorf("workflows[1].target_error: got %v", broken["target_error"])
	}

	get := mock.FindRequest("GET", "/gitops/workflows")
	if !strings.Contains(get.Query, "endpointIds=3%2C4") {
		t.Errorf("endpoint_ids should be sent as a comma-separated list, got %q", get.Query)
	}
	if !strings.Contains(get.Query, "platform=kubernetes") {
		t.Errorf("query should carry the platform filter, got %q", get.Query)
	}
}

// TestDataSourceGitopsWorkflow_Single verifies the single-workflow lookup.
func TestDataSourceGitopsWorkflow_Single(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/gitops/workflows/5", RespondJSON(http.StatusOK, map[string]interface{}{
		"id": 5, "name": "web", "creationDate": 1756800000, "lastSyncDate": 1756900000,
		"status": map[string]interface{}{
			"source":   map[string]interface{}{"status": "healthy"},
			"artifact": map[string]interface{}{"status": "syncing"},
			"target":   map[string]interface{}{"status": "healthy"},
		},
	}))

	ds := dataSourceGitopsWorkflow()
	d := ds.TestResourceData()
	_ = d.Set("workflow_id", 5)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "5" {
		t.Errorf("id: got %q", d.Id())
	}
	if got := d.Get("artifact_status"); got != "syncing" {
		t.Errorf("artifact_status: got %v", got)
	}
	if got := d.Get("healthy"); got != false {
		t.Errorf("healthy: expected false while a phase is syncing, got %v", got)
	}
}

// TestDataSourceGitopsSourceConnection_AdHoc verifies the ad-hoc test posts the
// URL to the unstored endpoint.
func TestDataSourceGitopsSourceConnection_AdHoc(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/gitops/sources/test", RespondJSON(http.StatusOK, map[string]interface{}{"success": true}))

	ds := dataSourceGitopsSourceConnection()
	d := ds.TestResourceData()
	_ = d.Set("url", "https://github.com/acme/infra.git")
	_ = d.Set("username", "ci")
	_ = d.Set("password", "token")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("success"); got != true {
		t.Errorf("success: got %v", got)
	}

	post := mock.FindRequest("POST", "/gitops/sources/test")
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if payload["url"] != "https://github.com/acme/infra.git" {
		t.Errorf("payload.url: got %v", payload["url"])
	}
}

// TestDataSourceGitopsSourceConnection_StoredSource verifies a source_id routes
// to the stored-source endpoint and that the URL is not sent there.
func TestDataSourceGitopsSourceConnection_StoredSource(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/gitops/sources/12/test", RespondJSON(http.StatusOK, map[string]interface{}{
		"success": false, "error": "authentication failed",
	}))

	ds := dataSourceGitopsSourceConnection()
	d := ds.TestResourceData()
	_ = d.Set("source_id", 12)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read should report a failed connection, not raise it: %v", err)
	}
	if got := d.Get("success"); got != false {
		t.Errorf("success: got %v", got)
	}
	if got := d.Get("error"); got != "authentication failed" {
		t.Errorf("error: got %v", got)
	}
}

// TestDataSourceGitopsSourceConnection_FailOnError verifies the opt-in failure
// mode, and that neither-or-both inputs are rejected.
func TestDataSourceGitopsSourceConnection_FailOnError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/gitops/sources/12/test", RespondJSON(http.StatusOK, map[string]interface{}{
		"success": false, "error": "authentication failed",
	}))

	ds := dataSourceGitopsSourceConnection()
	d := ds.TestResourceData()
	_ = d.Set("source_id", 12)
	_ = d.Set("fail_on_error", true)

	err := rcRead(ds, d, mock.Client())
	if err == nil {
		t.Fatal("expected Read to fail when fail_on_error is set and the test failed")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("error should carry the server's reason, got: %v", err)
	}

	empty := dataSourceGitopsSourceConnection()
	de := empty.TestResourceData()
	if err := rcRead(empty, de, mock.Client()); err == nil {
		t.Error("expected Read to fail when neither source_id nor url is set")
	}
}

// TestDataSourceGitopsWorkflow_SchemaMatchesList guards the one duplication in
// the GitOps data sources: portainer_gitops_workflow spells its schema out so
// docsdriftlint can read it statically, while portainer_gitops_workflows builds
// the same attributes from gitopsWorkflowAttributes(). A field added to one and
// not the other would silently stop being reported by the single-workflow
// lookup, since both are filled from the same gitopsWorkflow.toMap().
func TestDataSourceGitopsWorkflow_SchemaMatchesList(t *testing.T) {
	single := dataSourceGitopsWorkflow().Schema
	shared := gitopsWorkflowAttributes()

	for name := range shared {
		if name == "id" {
			// Carried by the resource ID on the single lookup.
			continue
		}
		if _, ok := single[name]; !ok {
			t.Errorf("portainer_gitops_workflow is missing %q, which portainer_gitops_workflows exposes", name)
		}
	}
	for name := range single {
		if name == "workflow_id" {
			continue
		}
		if _, ok := shared[name]; !ok {
			t.Errorf("portainer_gitops_workflow exposes %q, which portainer_gitops_workflows does not", name)
		}
	}
}
