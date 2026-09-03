package internal

import (
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// TestDataSourceDockerDashboard_HappyPath verifies the nested counters are
// flattened onto the data source.
func TestDataSourceDockerDashboard_HappyPath(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/docker/1/dashboard", RespondJSON(http.StatusOK, map[string]interface{}{
		"containers": map[string]interface{}{"total": 10, "running": 7, "stopped": 3, "healthy": 5, "unhealthy": 1},
		"images":     map[string]interface{}{"total": 22, "size": 1073741824},
		"volumes":    4, "networks": 3, "services": 0, "stacks": 2,
	}))

	ds := dataSourceDockerDashboard()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 1)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	for field, want := range map[string]interface{}{
		"containers_total": 10, "containers_running": 7, "containers_unhealthy": 1,
		"images_total": 22, "images_size": 1073741824, "volumes": 4, "stacks": 2,
	} {
		if got := d.Get(field); got != want {
			t.Errorf("%s: expected %v, got %v", field, want, got)
		}
	}
}

// TestDataSourceDockerImages_UsageAccounting verifies with_usage reaches the
// API and that the unused tally only counts when it was asked for.
func TestDataSourceDockerImages_UsageAccounting(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/docker/1/images", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"id": "sha256:aaa", "tags": []string{"nginx:1.27"}, "size": 100, "created": 1756900000, "used": true},
		{"id": "sha256:bbb", "size": 50, "created": 1756800000, "used": false},
	}))

	ds := dataSourceDockerImages()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 1)
	_ = d.Set("with_usage", true)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("total_size"); got != 150 {
		t.Errorf("total_size: expected 150, got %v", got)
	}
	if got := d.Get("unused_count"); got != 1 {
		t.Errorf("unused_count: expected 1, got %v", got)
	}
	// A dangling image has no tags; the list must still be present, not nil.
	dangling := d.Get("images").([]interface{})[1].(map[string]interface{})
	if dangling["tags"] == nil {
		t.Error("tags should be an empty list rather than nil for a dangling image")
	}
	if get := mock.FindRequest("GET", "/docker/1/images"); !strings.Contains(get.Query, "withUsage=true") {
		t.Errorf("query should carry withUsage, got %q", get.Query)
	}
}

// TestDataSourceDockerImages_WithoutUsageDoesNotCount guards the caveat in the
// docs: without with_usage, `used` is always false and must not be read as
// "nothing is running".
func TestDataSourceDockerImages_WithoutUsageDoesNotCount(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/docker/1/images", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"id": "sha256:aaa", "size": 100, "used": false},
	}))

	ds := dataSourceDockerImages()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 1)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("unused_count"); got != 0 {
		t.Errorf("unused_count must stay 0 when usage was not requested, got %v", got)
	}
	if get := mock.FindRequest("GET", "/docker/1/images"); get.Query != "" {
		t.Errorf("query should be empty when with_usage is off, got %q", get.Query)
	}
}

// TestDataSourceAppTemplates_ListAndFile verifies the catalogue is mapped and
// that a template_id triggers the file lookup, which Portainer models as a POST.
func TestDataSourceAppTemplates_ListAndFile(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/templates", RespondJSON(http.StatusOK, map[string]interface{}{
		"version": "3",
		"templates": []map[string]interface{}{
			{
				"Id": 12, "title": "WordPress", "description": "Blog", "type": 3,
				"platform": "linux", "categories": []string{"cms"},
				"repository": map[string]interface{}{"url": "https://github.com/acme/templates"},
			},
		},
	}))
	mock.On("POST", "/templates/12/file", RespondJSON(http.StatusOK, map[string]interface{}{
		"FileContent": "version: '3'\n",
	}))

	ds := dataSourceAppTemplates()
	d := ds.TestResourceData()
	_ = d.Set("template_id", 12)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	tmpl := d.Get("templates").([]interface{})[0].(map[string]interface{})
	if tmpl["title"] != "WordPress" || tmpl["repository_url"] != "https://github.com/acme/templates" {
		t.Errorf("templates[0] mismatch: %v", tmpl)
	}
	if got := d.Get("file_content"); got != "version: '3'\n" {
		t.Errorf("file_content: got %q", got)
	}
}

// TestDataSourceAppTemplates_WithoutTemplateID verifies the file endpoint is
// left alone when no template is named.
func TestDataSourceAppTemplates_WithoutTemplateID(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/templates", RespondJSON(http.StatusOK, map[string]interface{}{
		"version": "3", "templates": []map[string]interface{}{},
	}))

	ds := dataSourceAppTemplates()
	d := ds.TestResourceData()

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("file_content"); got != "" {
		t.Errorf("file_content should stay empty, got %q", got)
	}
}

// TestDataSourceHelmChart_IndexAndChart verifies both shapes: the repository
// index when no chart is named, and the per-chart command otherwise.
func TestDataSourceHelmChart_IndexAndChart(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/templates/helm", RespondString(http.StatusOK, "text/yaml", "apiVersion: v1\nentries: {}\n"))
	mock.On("GET", "/templates/helm/values", RespondString(http.StatusOK, "text/yaml", "replicaCount: 1\n"))

	index := dataSourceHelmChart()
	di := index.TestResourceData()
	_ = di.Set("repo", "https://charts.example.com")
	if err := rcRead(index, di, mock.Client()); err != nil {
		t.Fatalf("Read (index) failed: %v", err)
	}
	if !strings.Contains(di.Get("content").(string), "entries") {
		t.Errorf("index content: got %q", di.Get("content"))
	}

	chart := dataSourceHelmChart()
	dc := chart.TestResourceData()
	_ = dc.Set("repo", "https://charts.example.com")
	_ = dc.Set("chart", "nginx")
	_ = dc.Set("command", "values")
	_ = dc.Set("version", "1.2.3")
	if err := rcRead(chart, dc, mock.Client()); err != nil {
		t.Fatalf("Read (chart) failed: %v", err)
	}
	if dc.Get("content") != "replicaCount: 1\n" {
		t.Errorf("chart content: got %q", dc.Get("content"))
	}
	get := mock.FindRequest("GET", "/templates/helm/values")
	for _, want := range []string{"chart=nginx", "version=1.2.3"} {
		if !strings.Contains(get.Query, want) {
			t.Errorf("query %q should contain %q", get.Query, want)
		}
	}
}

// TestDataSourceStoredFiles verifies the three file endpoints, which differ
// only in the JSON field their body uses.
func TestDataSourceStoredFiles(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/custom_templates/3/file", RespondJSON(http.StatusOK, map[string]interface{}{"FileContent": "custom"}))
	mock.On("GET", "/edge_stacks/4/file", RespondJSON(http.StatusOK, map[string]interface{}{"StackFileContent": "edge stack"}))
	mock.On("GET", "/edge_jobs/5/file", RespondJSON(http.StatusOK, map[string]interface{}{"FileContent": "#!/bin/sh"}))

	ct := dataSourceCustomTemplateFile()
	dct := ct.TestResourceData()
	_ = dct.Set("template_id", 3)
	if err := rcRead(ct, dct, mock.Client()); err != nil {
		t.Fatalf("custom template file: %v", err)
	}
	if dct.Get("file_content") != "custom" {
		t.Errorf("custom template file_content: got %v", dct.Get("file_content"))
	}

	es := dataSourceEdgeStackFile()
	des := es.TestResourceData()
	_ = des.Set("edge_stack_id", 4)
	if err := rcRead(es, des, mock.Client()); err != nil {
		t.Fatalf("edge stack file: %v", err)
	}
	// The edge stack endpoint uses StackFileContent, not FileContent.
	if des.Get("file_content") != "edge stack" {
		t.Errorf("edge stack file_content: got %v", des.Get("file_content"))
	}

	ej := dataSourceEdgeJobFile()
	dej := ej.TestResourceData()
	_ = dej.Set("edge_job_id", 5)
	if err := rcRead(ej, dej, mock.Client()); err != nil {
		t.Fatalf("edge job file: %v", err)
	}
	if dej.Get("file_content") != "#!/bin/sh" {
		t.Errorf("edge job file_content: got %v", dej.Get("file_content"))
	}
}

// TestDataSourceEdgeJobTasks_LogsCollected verifies the logs_collected flag,
// which is what tells a configuration the logs are there to read.
func TestDataSourceEdgeJobTasks_LogsCollected(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_jobs/1/tasks", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": "1_2", "EndpointId": 2, "EndpointName": "branch-a", "LogsStatus": 3},
		{"Id": "1_3", "EndpointId": 3, "EndpointName": "branch-b", "LogsStatus": 1},
	}))

	ds := dataSourceEdgeJobTasks()
	d := ds.TestResourceData()
	_ = d.Set("edge_job_id", 1)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	tasks := d.Get("tasks").([]interface{})
	if tasks[0].(map[string]interface{})["logs_collected"] != true {
		t.Error("status 3 must be reported as collected")
	}
	if tasks[1].(map[string]interface{})["logs_collected"] != false {
		t.Error("status 1 must not be reported as collected")
	}
}

// TestDataSourceEdgeJobTaskLogs_NotCollectedYet verifies that logs which have
// not been collected read as empty rather than failing a plan.
func TestDataSourceEdgeJobTaskLogs_NotCollectedYet(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_jobs/1/tasks/1_3/logs",
		RespondString(http.StatusNotFound, "application/json", `{"message":"logs not found"}`))

	ds := dataSourceEdgeJobTaskLogs()
	d := ds.TestResourceData()
	_ = d.Set("edge_job_id", 1)
	_ = d.Set("task_id", "1_3")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("uncollected logs must not fail the read: %v", err)
	}
	if got := d.Get("logs"); got != "" {
		t.Errorf("logs: expected empty, got %q", got)
	}
}

// TestEdgeJobTaskLogs_CollectAndClear verifies the resource's two calls.
func TestEdgeJobTaskLogs_CollectAndClear(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/edge_jobs/1/tasks/1_2/logs", RespondString(http.StatusNoContent, "", ""))
	mock.On("DELETE", "/edge_jobs/1/tasks/1_2/logs", RespondString(http.StatusNoContent, "", ""))

	r := resourceEdgeJobTaskLogs()
	d := r.TestResourceData()
	_ = d.Set("edge_job_id", 1)
	_ = d.Set("task_id", "1_2")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "1/1_2/logs" {
		t.Errorf("id: got %q", d.Id())
	}
	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/edge_jobs/1/tasks/1_2/logs") == nil {
		t.Error("expected the logs to be cleared on destroy")
	}
}

// TestDataSourceEndpointsSummary_HealthCounters verifies the nested byHealth
// and byPlatformType objects are flattened.
func TestDataSourceEndpointsSummary_HealthCounters(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/summary", RespondJSON(http.StatusOK, map[string]interface{}{
		"total": 12, "unassigned": 2,
		"byHealth":       map[string]interface{}{"up": 9, "down": 3, "heartbeat": 5, "outdated": 1},
		"byPlatformType": map[string]interface{}{"docker": 7, "kubernetes": 4, "podman": 1},
		"byGroup": []map[string]interface{}{
			{"groupId": 1, "groupName": "Unassigned", "count": 2},
			{"groupId": 2, "groupName": "production", "count": 10},
		},
	}))

	ds := dataSourceEndpointsSummary()
	d := ds.TestResourceData()
	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	for field, want := range map[string]interface{}{
		"total": 12, "up": 9, "down": 3, "heartbeat": 5, "outdated": 1,
		"unassigned": 2, "docker": 7, "kubernetes": 4, "podman": 1,
	} {
		if got := d.Get(field); got != want {
			t.Errorf("%s: expected %v, got %v", field, want, got)
		}
	}
	if groups := d.Get("by_group").([]interface{}); len(groups) != 2 {
		t.Errorf("by_group: expected 2 entries, got %d", len(groups))
	}
}

// TestDataSourceEndpointRegistries_OmitsCredentials is the important one: the
// API hands back the full registry object including its password and access
// token, and neither may reach Terraform state.
func TestDataSourceEndpointRegistries_OmitsCredentials(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/1/registries", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"Id": 4, "Name": "harbor", "URL": "registry.example.com", "BaseURL": "https://registry.example.com",
			"Type": 3, "Authentication": true, "Username": "robot",
			"Password": "super-secret", "AccessToken": "tok-secret",
		},
	}))

	ds := dataSourceEndpointRegistries()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 1)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	reg := d.Get("registries").([]interface{})[0].(map[string]interface{})
	if reg["name"] != "harbor" || reg["username"] != "robot" || reg["authentication"] != true {
		t.Errorf("registry mismatch: %v", reg)
	}
	for _, forbidden := range []string{"password", "access_token", "Password", "AccessToken"} {
		if _, present := reg[forbidden]; present {
			t.Errorf("registry credentials must never be mapped into state, found %q", forbidden)
		}
	}
	if _, present := dataSourceEndpointRegistries().Schema["registries"].Elem.(*schema.Resource).Schema["password"]; present {
		t.Error("the schema must not even declare a password field")
	}
}

// TestDataSourceEndpointRegistries_DockerHubRateLimit verifies the optional
// second lookup only happens when asked for.
func TestDataSourceEndpointRegistries_DockerHubRateLimit(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/1/registries", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("GET", "/endpoints/1/dockerhub/6", RespondJSON(http.StatusOK, map[string]interface{}{
		"limit": 200, "remaining": 47,
	}))

	ds := dataSourceEndpointRegistries()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 1)
	_ = d.Set("dockerhub_registry_id", 6)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("dockerhub_rate_remaining"); got != 47 {
		t.Errorf("dockerhub_rate_remaining: got %v", got)
	}

	plain := dataSourceEndpointRegistries()
	dp := plain.TestResourceData()
	_ = dp.Set("environment_id", 1)
	if err := rcRead(plain, dp, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := dp.Get("dockerhub_rate_limit"); got != 0 {
		t.Errorf("the rate limit must stay zero when not requested, got %v", got)
	}
}

// TestDataSourceTeamMemberships_Leaders verifies leaders are picked out of the
// membership list by role.
func TestDataSourceTeamMemberships_Leaders(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/teams/1/memberships", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "UserID": 5, "Role": 1},
		{"Id": 2, "UserID": 6, "Role": 2},
		{"Id": 3, "UserID": 7, "Role": 1},
	}))

	ds := dataSourceTeamMemberships()
	d := ds.TestResourceData()
	_ = d.Set("team_id", 1)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if m := d.Get("memberships").([]interface{}); len(m) != 3 {
		t.Fatalf("memberships: expected 3, got %d", len(m))
	}
	leaders := d.Get("leader_user_ids").([]interface{})
	if len(leaders) != 2 || leaders[0] != 5 || leaders[1] != 7 {
		t.Errorf("leader_user_ids: expected [5 7], got %v", leaders)
	}
}
