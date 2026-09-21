package internal

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// =========================================================================
// Wave three, remainder: licensing, recommendations, policies, observability,
// Docker snapshots, image status, edge update schedules and the git helpers.
// =========================================================================

// TestDataSourceLicensesInfo_ReadsOveruse covers the field worth alerting on:
// a non-zero overuse timestamp means the instance is over its node count.
func TestDataSourceLicensesInfo_ReadsOveruse(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/licenses/info", RespondJSON(http.StatusOK, map[string]interface{}{
		"valid": true, "type": 2, "company": "Example Ltd", "nodes": 50,
		"expiresAt": 1800000000, "enforcedAt": 0, "overuseStartedTimestamp": 1756000000,
	}))

	ds := dataSourceLicensesInfo()
	d := ds.TestResourceData()
	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("nodes"); got != 50 {
		t.Errorf("nodes: got %v", got)
	}
	if got := d.Get("overuse_started_at"); got != 1756000000 {
		t.Errorf("overuse_started_at: got %v", got)
	}
	if got := d.Get("enforced_at"); got != 0 {
		t.Errorf("enforced_at: expected zero when nothing is enforced, got %v", got)
	}
}

// TestDataSourceRecommendations_FiltersListButNotSummary pins the distinction:
// the filters narrow the listing, the summary is over everything.
func TestDataSourceRecommendations_FiltersListButNotSummary(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/recommendations", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"typeId": "backup-not-configured", "title": "Configure backups",
			"description": "No backup schedule", "category": "resilience", "severity": "high",
			"actionLabel": "Configure", "actionUrl": "/settings/backup",
		},
	}))
	mock.On("GET", "/recommendations/summary", RespondJSON(http.StatusOK, map[string]interface{}{
		"total": 9, "totalTypes": 4,
		"severityCounts": map[string]int{"high": 2, "low": 7},
		"categoryCounts": map[string]int{"resilience": 3, "security": 6},
	}))

	ds := dataSourceRecommendations()
	d := ds.TestResourceData()
	_ = d.Set("severity", "high")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	req := mock.FindRequest("GET", "/recommendations")
	if req == nil || !strings.Contains(req.Query, "severity=high") {
		t.Errorf("expected the severity filter in the query, got %q", req.Query)
	}

	list := d.Get("recommendations").([]interface{})
	if len(list) != 1 {
		t.Fatalf("expected one recommendation, got %d", len(list))
	}
	if list[0].(map[string]interface{})["type_id"] != "backup-not-configured" {
		t.Errorf("the recommendation was not read: %+v", list[0])
	}

	// The summary counts every recommendation, not just the filtered ones.
	if got := d.Get("total"); got != 9 {
		t.Errorf("total: expected the unfiltered figure, got %v", got)
	}
	counts := d.Get("severity_counts").(map[string]interface{})
	if len(counts) != 2 {
		t.Errorf("the severity counts were not read: %v", counts)
	}
}

// TestDataSourcePolicyConflicts_PassesPolicyThrough pins the JSON passthrough,
// which is what Portainer's untyped payload forces.
func TestDataSourcePolicyConflicts_PassesPolicyThrough(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/policies/conflicts", RespondJSON(http.StatusOK, map[string]interface{}{
		"totalEnvironments": 10, "supportedEnvironments": 8, "unsupportedEnvironments": 2,
		"conflicts": []map[string]interface{}{
			{
				"environmentGroupId": 3, "environmentGroupName": "shops",
				"environmentCount": 5, "supportedEnvironments": 4, "unsupportedEnvironments": 1,
				"existingPolicyId": 7, "existingPolicyName": "baseline",
			},
		},
		"newGroups": []map[string]interface{}{
			{
				"environmentGroupId": 4, "environmentGroupName": "warehouses",
				"environmentCount": 5, "supportedEnvironments": 4, "unsupportedEnvironments": 1,
			},
		},
	}))

	ds := dataSourcePolicyConflicts()
	d := ds.TestResourceData()
	_ = d.Set("policy_json", `{"name":"baseline-v2","environmentGroupIds":[3,4]}`)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	var payload map[string]interface{}
	if err := mock.FindRequest("POST", "/policies/conflicts").DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if payload["name"] != "baseline-v2" {
		t.Errorf("the candidate policy must be sent as given, got %v", payload)
	}

	if got := d.Get("unsupported_environments"); got != 2 {
		t.Errorf("unsupported_environments: got %v", got)
	}
	conflicts := d.Get("conflicts").([]interface{})
	if len(conflicts) != 1 || conflicts[0].(map[string]interface{})["existing_policy_name"] != "baseline" {
		t.Errorf("the conflicts were not read: %v", conflicts)
	}
	groups := d.Get("new_groups").([]interface{})
	if len(groups) != 1 || groups[0].(map[string]interface{})["environment_group_name"] != "warehouses" {
		t.Errorf("the new groups were not read: %v", groups)
	}
}

// TestDataSourcePolicyObservabilityTest_OmitsUnsetCredentials keeps the test
// from sending a blank key OneUptime would reject for the wrong reason.
func TestDataSourcePolicyObservabilityTest_OmitsUnsetCredentials(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/policies/observability-k8s/test", RespondJSON(http.StatusOK, map[string]interface{}{
		"success": true, "message": "connected",
	}))

	ds := dataSourcePolicyObservabilityTest()
	d := ds.TestResourceData()
	_ = d.Set("one_uptime_url", "https://oneuptime.example.com")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	var payload map[string]interface{}
	if err := mock.FindRequest("POST", "/policies/observability-k8s/test").DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	for _, key := range []string{"apiKey", "projectID", "policyId"} {
		if _, ok := payload[key]; ok {
			t.Errorf("%s must be omitted when unset, got %v", key, payload[key])
		}
	}
	if got := d.Get("success"); got != true {
		t.Errorf("success: got %v", got)
	}
}

// TestDataSourcePolicyObservabilityTest_FailsByDefault covers the pre-flight
// default: a failed connection stops the plan.
func TestDataSourcePolicyObservabilityTest_FailsByDefault(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/policies/observability-k8s/test", RespondJSON(http.StatusOK, map[string]interface{}{
		"success": false, "message": "project not found",
	}))

	ds := dataSourcePolicyObservabilityTest()
	d := ds.TestResourceData()
	_ = d.Set("one_uptime_url", "https://oneuptime.example.com")
	_ = d.Set("fail_on_error", true)

	err := rcRead(ds, d, mock.Client())
	if err == nil {
		t.Fatal("a failed connection must stop the plan by default")
	}
	if !strings.Contains(err.Error(), "project not found") {
		t.Errorf("the error should carry what Portainer reported, got: %v", err)
	}
}

// TestDataSourceAlertingConnectivity_ReportsWhenAsked covers the switch that
// turns an unreachable Alertmanager into a readable answer.
func TestDataSourceAlertingConnectivity_ReportsWhenAsked(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/observability/alerting/connectivity", RespondString(http.StatusBadGateway,
		"application/json", `{"message":"dial tcp: connection refused"}`))

	ds := dataSourceAlertingConnectivity()
	d := ds.TestResourceData()
	_ = d.Set("url", "http://alertmanager.example.com:9093")
	_ = d.Set("fail_on_error", false)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read must not fail when fail_on_error is off: %v", err)
	}
	if got := d.Get("reachable"); got != false {
		t.Errorf("reachable: expected false, got %v", got)
	}
	if d.Get("error").(string) == "" {
		t.Error("the failure reason must be reported")
	}

	req := mock.FindRequest("GET", "/observability/alerting/connectivity")
	if req == nil || !strings.Contains(req.Query, "url=http") {
		t.Errorf("expected the Alertmanager URL in the query, got %q", req.Query)
	}
}

// TestDataSourceEnvironmentLogs_BuildsQuery pins the window and the filters,
// which are the whole interface of this data source.
func TestDataSourceEnvironmentLogs_BuildsQuery(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/observability/environments/5/logs", RespondJSON(http.StatusOK, map[string]interface{}{
		"logs": []map[string]interface{}{
			{
				"time": "2026-01-01T00:00:00Z", "severity": "error", "source": "web-0",
				"message": "connection refused", "labels": map[string]string{"app": "web"},
			},
		},
	}))

	ds := dataSourceEnvironmentLogs()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 5)
	_ = d.Set("from", "2026-01-01T00:00:00Z")
	_ = d.Set("to", "2026-01-02T00:00:00Z")
	_ = d.Set("namespace", "apps")
	_ = d.Set("severity", "error")
	_ = d.Set("limit", 100)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	query := mock.FindRequest("GET", "/observability/environments/5/logs").Query
	for _, want := range []string{"from=2026", "to=2026", "namespace=apps", "severity=error", "limit=100"} {
		if !strings.Contains(query, want) {
			t.Errorf("expected %q in the query, got %q", want, query)
		}
	}
	// skip was not set, so it must not be sent.
	if strings.Contains(query, "skip=") {
		t.Errorf("an unset paging offset must not be sent, got %q", query)
	}

	logs := d.Get("logs").([]interface{})
	if len(logs) != 1 {
		t.Fatalf("expected one log line, got %d", len(logs))
	}
	entry := logs[0].(map[string]interface{})
	if entry["message"] != "connection refused" || entry["severity"] != "error" {
		t.Errorf("the log line was not read: %+v", entry)
	}
	if labels := entry["labels"].(map[string]interface{}); labels["app"] != "web" {
		t.Errorf("the labels were not read: %v", labels)
	}
}

// TestDataSourceEnvironmentMetrics_CarriesSeriesAsJSON pins the decision to
// pass the series through: their shape depends on the metric and the grouping.
func TestDataSourceEnvironmentMetrics_CarriesSeriesAsJSON(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/observability/environments/5/metrics", RespondJSON(http.StatusOK, map[string]interface{}{
		"data": []map[string]interface{}{{"namespace": "apps", "value": 0.42}},
	}))

	ds := dataSourceEnvironmentMetrics()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 5)
	_ = d.Set("metric", "cpu")
	_ = d.Set("aggregation", "avg")
	_ = d.Set("from", "2026-01-01T00:00:00Z")
	_ = d.Set("to", "2026-01-02T00:00:00Z")
	_ = d.Set("group_by", "namespace")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	query := mock.FindRequest("GET", "/observability/environments/5/metrics").Query
	for _, want := range []string{"metric=cpu", "aggregation=avg", "groupBy=namespace"} {
		if !strings.Contains(query, want) {
			t.Errorf("expected %q in the query, got %q", want, query)
		}
	}
	if !strings.Contains(d.Get("data").(string), "0.42") {
		t.Errorf("the series must be carried through, got %v", d.Get("data"))
	}
}

// TestDataSourceEnvironmentMetrics_EmptySeries covers a window with no data,
// which must read as an empty JSON array rather than an empty string.
func TestDataSourceEnvironmentMetrics_EmptySeries(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/observability/environments/5/metrics", RespondJSON(http.StatusOK, map[string]interface{}{}))

	ds := dataSourceEnvironmentMetrics()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 5)
	_ = d.Set("metric", "cpu")
	_ = d.Set("aggregation", "avg")
	_ = d.Set("from", "a")
	_ = d.Set("to", "b")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("data"); got != "[]" {
		t.Errorf("an empty result must still decode as a JSON array, got %q", got)
	}
}

func snapshotContainerResponse() map[string]interface{} {
	return map[string]interface{}{
		"Id": "abc123", "Names": []string{"/web"}, "Image": "nginx:1.27",
		"ImageID": "sha256:deadbeef", "Command": "nginx -g daemon off;",
		"Created": 1756000000, "State": "running", "Status": "Up 3 hours",
		"Labels": map[string]string{"app": "web"},
		"Ports": []map[string]interface{}{
			{"IP": "0.0.0.0", "PrivatePort": 80, "PublicPort": 8080, "Type": "tcp"},
		},
		"Mounts": []map[string]interface{}{{"Source": "/data"}},
	}
}

// TestDataSourceDockerSnapshotContainers_AcceptsBothShapes covers a real
// mismatch: Portainer's specification types this listing as a single container
// even though the endpoint lists them, so both shapes have to be accepted.
func TestDataSourceDockerSnapshotContainers_AcceptsBothShapes(t *testing.T) {
	ds := dataSourceDockerSnapshotContainers()

	listMock := NewMockServer(t)
	listMock.On("GET", "/docker/5/snapshot/containers",
		RespondJSON(http.StatusOK, []interface{}{snapshotContainerResponse()}))
	dList := ds.TestResourceData()
	_ = dList.Set("endpoint_id", 5)
	if err := rcRead(ds, dList, listMock.Client()); err != nil {
		t.Fatalf("array response failed: %v", err)
	}
	containers := dList.Get("containers").([]interface{})
	if len(containers) != 1 {
		t.Fatalf("expected one container, got %d", len(containers))
	}
	container := containers[0].(map[string]interface{})
	if container["state"] != "running" || container["image"] != "nginx:1.27" {
		t.Errorf("the container was not read: %+v", container)
	}
	ports := container["ports"].([]interface{})
	if len(ports) != 1 || ports[0].(map[string]interface{})["public_port"] != 8080 {
		t.Errorf("the ports were not read: %v", ports)
	}

	singleMock := NewMockServer(t)
	singleMock.On("GET", "/docker/5/snapshot/containers",
		RespondJSON(http.StatusOK, snapshotContainerResponse()))
	dSingle := ds.TestResourceData()
	_ = dSingle.Set("endpoint_id", 5)
	if err := rcRead(ds, dSingle, singleMock.Client()); err != nil {
		t.Fatalf("a single-object response must be accepted too: %v", err)
	}
	if got := dSingle.Get("containers").([]interface{}); len(got) != 1 {
		t.Errorf("expected the single container to be wrapped in a list, got %v", got)
	}
}

// TestDataSourceDockerSnapshotContainers_EdgeStackFilter pins the one query
// parameter the listing takes.
func TestDataSourceDockerSnapshotContainers_EdgeStackFilter(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/docker/5/snapshot/containers", RespondJSON(http.StatusOK, []interface{}{}))

	ds := dataSourceDockerSnapshotContainers()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 5)
	_ = d.Set("edge_stack_id", 9)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if query := mock.FindRequest("GET", "/docker/5/snapshot/containers").Query; !strings.Contains(query, "edgeStackId=9") {
		t.Errorf("expected the edge stack filter in the query, got %q", query)
	}
}

// TestDataSourceDockerSnapshotContainer_KeepsRawDetails covers the single
// container view and the deep Docker structures it does not flatten.
func TestDataSourceDockerSnapshotContainer_KeepsRawDetails(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/docker/5/snapshot/containers/abc123",
		RespondJSON(http.StatusOK, snapshotContainerResponse()))

	ds := dataSourceDockerSnapshotContainer()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 5)
	_ = d.Set("container_id", "abc123")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("status"); got != "Up 3 hours" {
		t.Errorf("status: got %v", got)
	}
	if names := d.Get("names").([]interface{}); len(names) != 1 || names[0] != "/web" {
		t.Errorf("names: got %v", names)
	}
	// Mounts are not flattened, so they have to survive in the raw details.
	if !strings.Contains(d.Get("details").(string), "/data") {
		t.Error("the mounts must be kept in the raw response")
	}
}

// TestDataSourceImageStatus_KindSelectsEndpoint pins the three endpoints this
// data source folds, including the stack one that has no environment in its
// path.
func TestDataSourceImageStatus_KindSelectsEndpoint(t *testing.T) {
	for _, tc := range []struct {
		kind       string
		resourceID string
		path       string
	}{
		{"container", "abc123", "/docker/5/containers/abc123/image_status"},
		{"service", "svc-1", "/docker/5/services/svc-1/image_status"},
		{"stack", "9", "/stacks/9/images_status"},
	} {
		mock := NewMockServer(t)
		mock.On("GET", tc.path, RespondJSON(http.StatusOK, map[string]interface{}{
			"Status": "outdated", "Message": "a newer image is published",
		}))

		ds := dataSourceImageStatus()
		d := ds.TestResourceData()
		_ = d.Set("kind", tc.kind)
		_ = d.Set("endpoint_id", 5)
		_ = d.Set("resource_id", tc.resourceID)
		_ = d.Set("refresh", true)

		if err := rcRead(ds, d, mock.Client()); err != nil {
			t.Fatalf("%s: Read failed: %v", tc.kind, err)
		}
		req := mock.FindRequest("GET", tc.path)
		if req == nil {
			t.Errorf("%s: expected the request to go to %s", tc.kind, tc.path)
			continue
		}
		if !strings.Contains(req.Query, "refresh=true") {
			t.Errorf("%s: expected the refresh flag, got %q", tc.kind, req.Query)
		}
		if got := d.Get("status"); got != "outdated" {
			t.Errorf("%s: status: got %v", tc.kind, got)
		}
	}
}

// TestDataSourceImageStatus_RequiresEndpointForContainer catches the
// configuration mistake the folded data source makes possible.
func TestDataSourceImageStatus_RequiresEndpointForContainer(t *testing.T) {
	mock := NewMockServer(t)

	ds := dataSourceImageStatus()
	d := ds.TestResourceData()
	_ = d.Set("kind", "container")
	_ = d.Set("resource_id", "abc123")

	err := rcRead(ds, d, mock.Client())
	if err == nil {
		t.Fatal("a container check without an environment must be reported")
	}
	if !strings.Contains(err.Error(), "endpoint_id") {
		t.Errorf("the error should name the missing argument, got: %v", err)
	}
}

// TestDataSourceEdgeUpdateScheduleInfo_FoldsTwoCalls covers the pair of
// argument-less endpoints that answer the same question.
func TestDataSourceEdgeUpdateScheduleInfo_FoldsTwoCalls(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_update_schedules/info", RespondJSON(http.StatusOK, map[string]interface{}{
		"MinAgentVersion": "2.19.0", "UpToDateCount": 12, "OutdatedCount": 3,
		"HasLocalTimeZone": true, "HasNoLocalTimeZone": false,
	}))
	mock.On("GET", "/edge_update_schedules/agent_versions",
		RespondJSON(http.StatusOK, []string{"2.21.0", "2.20.0", "2.19.0"}))

	ds := dataSourceEdgeUpdateScheduleInfo()
	d := ds.TestResourceData()
	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("outdated_count"); got != 3 {
		t.Errorf("outdated_count: got %v", got)
	}
	versions := d.Get("agent_versions").([]interface{})
	// Sorted, so two identical plans produce identical output.
	if len(versions) != 3 || versions[0] != "2.19.0" {
		t.Errorf("the agent versions must be sorted, got %v", versions)
	}
}

// TestDataSourceEdgeUpdatePreviousVersions_SortsAndFilters covers the map
// Portainer answers with and the comma-joined identifier filters.
func TestDataSourceEdgeUpdatePreviousVersions_SortsAndFilters(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_update_schedules/previous_versions", RespondJSON(http.StatusOK, map[string]string{
		"12": "2.20.0", "3": "2.19.0",
	}))

	ds := dataSourceEdgeUpdatePreviousVersions()
	d := ds.TestResourceData()
	_ = d.Set("environment_ids", []interface{}{3, 12})

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if query := mock.FindRequest("GET", "/edge_update_schedules/previous_versions").Query; !strings.Contains(query, "environmentIds=3%2C12") {
		t.Errorf("expected the identifiers joined with a comma, got %q", query)
	}
	entries := d.Get("previous_versions").([]interface{})
	if len(entries) != 2 {
		t.Fatalf("expected two entries, got %d", len(entries))
	}
	// Sorted by identifier as a string, which is what keeps the output stable.
	if entries[0].(map[string]interface{})["endpoint_id"] != "12" {
		t.Errorf("the entries must be sorted, got %v", entries)
	}
}

// TestDataSourceEdgeUpdateSchedulesActive_PostsEnvironments pins the one read
// in this group that Portainer exposes as a POST.
func TestDataSourceEdgeUpdateSchedulesActive_PostsEnvironments(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/edge_update_schedules/active", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"scheduleId": 2, "environmentId": 12, "edgeStackId": 8, "targetVersion": "2.21.0"},
	}))

	ds := dataSourceEdgeUpdateSchedulesActive()
	d := ds.TestResourceData()
	_ = d.Set("environment_ids", []interface{}{12, 13})

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	var payload struct {
		EnvironmentIDs []int `json:"EnvironmentIDs"`
	}
	if err := mock.FindRequest("POST", "/edge_update_schedules/active").DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if len(payload.EnvironmentIDs) != 2 {
		t.Errorf("the environments were not sent: %v", payload.EnvironmentIDs)
	}

	schedules := d.Get("schedules").([]interface{})
	if len(schedules) != 1 || schedules[0].(map[string]interface{})["target_version"] != "2.21.0" {
		t.Errorf("the active schedules were not read: %v", schedules)
	}
}

// TestDataSourceEdgeConfigurationFiles_CarriesPayload pins that this endpoint
// hands back the payload itself rather than a JSON document.
func TestDataSourceEdgeConfigurationFiles_CarriesPayload(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_configurations/4/files", RespondString(http.StatusOK,
		"application/octet-stream", "BASE_DOMAIN=apps.example.com\n"))

	ds := dataSourceEdgeConfigurationFiles()
	d := ds.TestResourceData()
	_ = d.Set("edge_configuration_id", 4)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if !strings.Contains(d.Get("files").(string), "BASE_DOMAIN") {
		t.Errorf("the payload must be carried through unchanged, got %q", d.Get("files"))
	}
}

// TestDataSourceEdgeStackStaggerStatus_Reads covers the stagger status read.
func TestDataSourceEdgeStackStaggerStatus_Reads(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_stacks/9/stagger/status", RespondJSON(http.StatusOK, map[string]interface{}{
		"status": "in-progress",
	}))

	ds := dataSourceEdgeStackStaggerStatus()
	d := ds.TestResourceData()
	_ = d.Set("edge_stack_id", 9)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("status"); got != "in-progress" {
		t.Errorf("status: got %v", got)
	}
}

// TestDataSourceGitopsRepoFileSearch_OmitsUnsetCredentials keeps a public
// repository from being cloned with a blank username and password.
func TestDataSourceGitopsRepoFileSearch_OmitsUnsetCredentials(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/gitops/repo/files/search", RespondJSON(http.StatusOK,
		[]string{"b/stack.yaml", "a/stack.yaml"}))

	ds := dataSourceGitopsRepoFileSearch()
	d := ds.TestResourceData()
	_ = d.Set("repository", "https://github.com/example/platform")
	_ = d.Set("reference", "refs/heads/main")
	_ = d.Set("include", "yml,yaml")
	_ = d.Set("force", true)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	req := mock.FindRequest("POST", "/gitops/repo/files/search")
	if !strings.Contains(req.Query, "force=true") {
		t.Errorf("expected the force flag in the query, got %q", req.Query)
	}
	var payload map[string]interface{}
	if err := req.DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	for _, key := range []string{"username", "password", "sourceId"} {
		if _, ok := payload[key]; ok {
			t.Errorf("%s must be omitted when unset, got %v", key, payload[key])
		}
	}
	if payload["include"] != "yml,yaml" {
		t.Errorf("the include filter was not sent: %v", payload["include"])
	}

	paths := d.Get("paths").([]interface{})
	if len(paths) != 2 || paths[0] != "a/stack.yaml" {
		t.Errorf("the paths must be sorted, got %v", paths)
	}
}

// TestDataSourceGitopsHelmValues_KeepsFileOrder is the one list in this block
// that must not be sorted: the order the files were merged in is what decides
// which value wins.
func TestDataSourceGitopsHelmValues_KeepsFileOrder(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/gitops/repo/helm/values", RespondJSON(http.StatusOK, map[string]interface{}{
		"mergedValues":   "replicas: 3\n",
		"filesProcessed": []string{"values.yaml", "values-prod.yaml"},
		"commitHash":     "abc123",
	}))

	ds := dataSourceGitopsHelmValues()
	d := ds.TestResourceData()
	_ = d.Set("repository", "https://github.com/example/platform")
	_ = d.Set("reference", "refs/heads/main")
	_ = d.Set("values_files", []interface{}{"values.yaml", "values-prod.yaml"})

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	var payload struct {
		ValuesFiles []string `json:"valuesFiles"`
	}
	if err := mock.FindRequest("POST", "/gitops/repo/helm/values").DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if len(payload.ValuesFiles) != 2 || payload.ValuesFiles[0] != "values.yaml" {
		t.Errorf("the values files must be sent in the configured order: %v", payload.ValuesFiles)
	}

	processed := d.Get("files_processed").([]interface{})
	if len(processed) != 2 || processed[0] != "values.yaml" || processed[1] != "values-prod.yaml" {
		t.Errorf("the processed order must be preserved, not sorted: %v", processed)
	}
	if got := d.Get("commit_hash"); got != "abc123" {
		t.Errorf("commit_hash: got %v", got)
	}
	if !strings.Contains(d.Get("merged_values").(string), "replicas: 3") {
		t.Errorf("the merged values were not read: %v", d.Get("merged_values"))
	}
}

// TestAPIGETRaw_FailsOnErrorStatus pins the reason apiGETRaw exists. Its
// neighbour apiGETCtx hands back the body of a 403 or a 500 with no error at
// all, which for a data source that stores the body verbatim would mean
// writing an error page into state and reporting success.
func TestAPIGETRaw_FailsOnErrorStatus(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/docker/5/snapshot", RespondString(http.StatusForbidden,
		"application/json", `{"message":"forbidden"}`))

	ds := dataSourceDockerSnapshot()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 5)

	err := rcRead(ds, d, mock.Client())
	if err == nil {
		t.Fatal("a forbidden response must fail the read rather than be stored as the snapshot")
	}
	if d.Get("snapshot").(string) != "" {
		t.Errorf("nothing must be written to state on failure, got %q", d.Get("snapshot"))
	}
}

// TestAPIGETRaw_NotFoundIsRecognised checks the error type carries through, so
// callers that tolerate a missing resource still can.
func TestAPIGETRaw_NotFoundIsRecognised(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/docker/5/snapshot", RespondString(http.StatusNotFound,
		"application/json", `{"message":"environment not found"}`))

	_, err := apiGETRaw(context.Background(), mock.Client(), mock.Client().Endpoint+"/docker/5/snapshot")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !isAPINotFound(err) {
		t.Errorf("a 404 must be recognisable as a missing resource, got: %v", err)
	}
}

// TestDataSourceAgentVersions_Sorts covers the fleet's agent versions, which
// is a different question from the versions an update schedule can move to.
func TestDataSourceAgentVersions_Sorts(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/agent_versions", RespondJSON(http.StatusOK,
		[]string{"2.21.0", "2.19.0", "2.20.0"}))

	ds := dataSourceAgentVersions()
	d := ds.TestResourceData()
	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	versions := d.Get("versions").([]interface{})
	if len(versions) != 3 || versions[0] != "2.19.0" {
		t.Errorf("the versions must be sorted, got %v", versions)
	}
}
