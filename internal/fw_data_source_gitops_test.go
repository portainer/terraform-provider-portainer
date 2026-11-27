package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// gitopsDSCheckRequest runs check against the first request sent to
// method+path.
func gitopsDSCheckRequest(mock *MockServer, method, path string, check func(*RecordedRequest) error) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		req := mock.FindRequest(method, path)
		if req == nil {
			return fmt.Errorf("expected %s %s to be sent", method, path)
		}
		return check(req)
	}
}

// gitopsDSPayload decodes a recorded JSON body.
func gitopsDSPayload(req *RecordedRequest) (map[string]interface{}, error) {
	var payload map[string]interface{}
	if err := json.Unmarshal(req.Body, &payload); err != nil {
		return nil, fmt.Errorf("payload is not JSON: %w", err)
	}
	return payload, nil
}

func gitopsDSUnitTest(t *testing.T, mock *MockServer, config string, check resource.TestCheckFunc) {
	t.Helper()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps:                    []resource.TestStep{{Config: mockProviderConfig(mock) + config, Check: check}},
	})
}

func gitopsDSUnitTestError(t *testing.T, mock *MockServer, config string, want *regexp.Regexp) {
	t.Helper()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps:                    []resource.TestStep{{Config: mockProviderConfig(mock) + config, ExpectError: want}},
	})
}

// --- portainer_gitops_repo_refs ---------------------------------------------

func TestGitopsRepoRefsDataSource_HappyPath(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/gitops/repo/refs", RespondJSON(http.StatusOK, []string{
		"refs/heads/main", "refs/heads/develop", "refs/tags/v1.0.0",
	}))

	gitopsDSUnitTest(t, mock, `data "portainer_gitops_repo_refs" "r" { repository_url = "https://github.com/owner/repo.git" }`,
		resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("data.portainer_gitops_repo_refs.r", "id", "gitops-repo-refs-https://github.com/owner/repo.git"),
			resource.TestCheckResourceAttr("data.portainer_gitops_repo_refs.r", "refs.#", "3"),
			resource.TestCheckResourceAttr("data.portainer_gitops_repo_refs.r", "refs.0", "refs/heads/main"),
			resource.TestCheckResourceAttr("data.portainer_gitops_repo_refs.r", "refs.2", "refs/tags/v1.0.0"),
			resource.TestCheckResourceAttr("data.portainer_gitops_repo_refs.r", "tls_skip_verify", "false"),
			resource.TestCheckNoResourceAttr("data.portainer_gitops_repo_refs.r", "username"),
			gitopsDSCheckRequest(mock, "POST", "/gitops/repo/refs", func(req *RecordedRequest) error {
				payload, err := gitopsDSPayload(req)
				if err != nil {
					return err
				}
				for _, key := range []string{"username", "password", "gitCredentialID", "TLSSkipVerify"} {
					if _, ok := payload[key]; ok {
						return fmt.Errorf("%s must be omitted when unset, got %v", key, payload[key])
					}
				}
				return nil
			}),
		))
}

func TestGitopsRepoRefsDataSource_PayloadShape(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/gitops/repo/refs", RespondJSON(http.StatusOK, []string{}))

	gitopsDSUnitTest(t, mock, `data "portainer_gitops_repo_refs" "r" {
  repository_url    = "https://example.com/repo.git"
  username          = "robot"
  password          = "secret"
  git_credential_id = 42
  tls_skip_verify   = true
}`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr("data.portainer_gitops_repo_refs.r", "refs.#", "0"),
		gitopsDSCheckRequest(mock, "POST", "/gitops/repo/refs", func(req *RecordedRequest) error {
			payload, err := gitopsDSPayload(req)
			if err != nil {
				return err
			}
			want := map[string]interface{}{
				"repository": "https://example.com/repo.git", "username": "robot", "password": "secret",
				"gitCredentialID": float64(42), "TLSSkipVerify": true,
			}
			for k, v := range want {
				if payload[k] != v {
					return fmt.Errorf("payload.%s: got %v, want %v", k, payload[k], v)
				}
			}
			return nil
		}),
	))
}

func TestGitopsRepoRefsDataSource_HTTPError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/gitops/repo/refs", RespondString(http.StatusBadRequest, "application/json", `{"message":"invalid repository URL"}`))

	gitopsDSUnitTestError(t, mock, `data "portainer_gitops_repo_refs" "r" { repository_url = "not-a-url" }`,
		regexp.MustCompile(`failed to list Git repository refs`))
}

// --- portainer_gitops_repo_file ---------------------------------------------

func TestGitopsRepoFileDataSource_HappyPath(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/gitops/repo/file/preview", RespondJSON(http.StatusOK, map[string]interface{}{
		"FileContent": "version: '3'\nservices: {}\n",
	}))

	gitopsDSUnitTest(t, mock, `data "portainer_gitops_repo_file" "f" {
  repository_url = "https://github.com/owner/repo.git"
  reference      = "refs/heads/main"
  target_file    = "docker-compose.yml"
  username       = "user"
  password       = "secret"
}`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr("data.portainer_gitops_repo_file.f", "id", "gitops-repo-file-https://github.com/owner/repo.git-docker-compose.yml"),
		resource.TestCheckResourceAttr("data.portainer_gitops_repo_file.f", "file_content", "version: '3'\nservices: {}\n"),
		gitopsDSCheckRequest(mock, "POST", "/gitops/repo/file/preview", func(req *RecordedRequest) error {
			payload, err := gitopsDSPayload(req)
			if err != nil {
				return err
			}
			want := map[string]interface{}{
				"repository": "https://github.com/owner/repo.git", "reference": "refs/heads/main",
				"targetFile": "docker-compose.yml", "username": "user", "password": "secret",
			}
			for k, v := range want {
				if payload[k] != v {
					return fmt.Errorf("payload.%s: got %v, want %v", k, payload[k], v)
				}
			}
			return nil
		}),
	))
}

func TestGitopsRepoFileDataSource_HTTPError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/gitops/repo/file/preview", RespondString(http.StatusUnauthorized, "application/json", `{"message":"unauthorized"}`))

	gitopsDSUnitTestError(t, mock, `data "portainer_gitops_repo_file" "f" { repository_url = "https://example.com/repo.git" }`,
		regexp.MustCompile(`failed to preview Git repository file`))
}

func TestGitopsRepoFileDataSource_BadJSON(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/gitops/repo/file/preview", RespondString(http.StatusOK, "application/json", `{not json`))

	gitopsDSUnitTestError(t, mock, `data "portainer_gitops_repo_file" "f" { repository_url = "https://example.com/repo.git" }`,
		regexp.MustCompile(`failed to preview Git repository file`))
}

// --- portainer_gitops_sources / portainer_gitops_source ---------------------

// TestGitopsSourcesDataSource_ListAndSummary: the list is mapped, the filters
// reach the API as query parameters, and the separate summary endpoint is
// folded into the same data source.
func TestGitopsSourcesDataSource_ListAndSummary(t *testing.T) {
	t.Parallel()
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

	const ds = "data.portainer_gitops_sources.s"
	gitopsDSUnitTest(t, mock, `data "portainer_gitops_sources" "s" {
  search = "acme"
  status = "healthy"
  limit  = 50
  start  = 0
}`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(ds, "sources.#", "2"),
		resource.TestCheckResourceAttr(ds, "sources.0.name", "infra"),
		resource.TestCheckResourceAttr(ds, "sources.0.used_by", "3"),
		resource.TestCheckResourceAttr(ds, "sources.0.environments", "2"),
		resource.TestCheckResourceAttr(ds, "sources.1.status", "error"),
		resource.TestCheckResourceAttr(ds, "sources.1.status_error", "authentication failed"),
		resource.TestCheckResourceAttr(ds, "summary.#", "1"),
		resource.TestCheckResourceAttr(ds, "summary.0.healthy", "1"),
		resource.TestCheckResourceAttr(ds, "summary.0.error", "1"),
		resource.TestMatchResourceAttr(ds, "id", regexp.MustCompile(`^gitops-sources-\d+$`)),
		gitopsDSCheckRequest(mock, "GET", "/gitops/sources", func(req *RecordedRequest) error {
			for _, want := range []string{"search=acme", "status=healthy", "limit=50"} {
				if !strings.Contains(req.Query, want) {
					return fmt.Errorf("query %q should contain %q", req.Query, want)
				}
			}
			// SDKv2 treated a zero as unset and never sent it.
			if strings.Contains(req.Query, "start=") {
				return fmt.Errorf("query %q should not carry start=0", req.Query)
			}
			return nil
		}),
	))
}

func TestGitopsSourcesDataSource_InvalidOrder(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	gitopsDSUnitTestError(t, mock, `data "portainer_gitops_sources" "s" { order = "up" }`,
		regexp.MustCompile(`value must be one of`))
}

// TestGitopsSourceDataSource_WithWorkflows: the single-source data source also
// resolves the workflows using it.
func TestGitopsSourceDataSource_WithWorkflows(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/gitops/sources/12", RespondJSON(http.StatusOK, map[string]interface{}{
		"id": 12, "name": "infra", "url": "https://github.com/acme/infra.git",
		"type": "git", "status": "healthy", "interval": "5m", "lastSync": 1756900000,
		"connection": map[string]interface{}{
			"tlsSkipVerify":  true,
			"authentication": map[string]interface{}{"username": "ci"},
		},
		"access": map[string]interface{}{"public": false, "teams": []int{4}, "users": []int{7, 9}},
	}))
	mock.On("GET", "/gitops/sources/12/workflows", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"id": 5, "name": "web", "type": "edgeStack", "platform": "kubernetes",
			"target":       map[string]interface{}{"namespace": "prod", "edgeGroupIds": []int{1, 2}},
			"creationDate": 1756800000, "lastSyncDate": 1756900000,
		},
		{"id": 6, "name": "bare"},
	}))

	const ds = "data.portainer_gitops_source.s"
	gitopsDSUnitTest(t, mock, `data "portainer_gitops_source" "s" { source_id = 12 }`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(ds, "id", "12"),
		resource.TestCheckResourceAttr(ds, "name", "infra"),
		resource.TestCheckResourceAttr(ds, "username", "ci"),
		resource.TestCheckResourceAttr(ds, "tls_skip_verify", "true"),
		resource.TestCheckResourceAttr(ds, "public", "false"),
		resource.TestCheckResourceAttr(ds, "status_error", ""),
		resource.TestCheckResourceAttr(ds, "user_accesses.#", "2"),
		resource.TestCheckResourceAttr(ds, "team_accesses.0", "4"),
		resource.TestCheckResourceAttr(ds, "workflows.#", "2"),
		resource.TestCheckResourceAttr(ds, "workflows.0.name", "web"),
		resource.TestCheckResourceAttr(ds, "workflows.0.platform", "kubernetes"),
		resource.TestCheckResourceAttr(ds, "workflows.0.namespace", "prod"),
		resource.TestCheckResourceAttr(ds, "workflows.0.edge_group_ids.#", "2"),
		resource.TestCheckResourceAttr(ds, "workflows.1.edge_group_ids.#", "0"),
		resource.TestCheckResourceAttr(ds, "workflows.1.endpoint_id", "0"),
	))
}

func TestGitopsSourceDataSource_NotFound(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/gitops/sources/12", RespondString(http.StatusNotFound, "application/json", `{"message":"not found"}`))
	gitopsDSUnitTestError(t, mock, `data "portainer_gitops_source" "s" { source_id = 12 }`,
		regexp.MustCompile(`failed to read GitOps source 12`))
}

// --- portainer_gitops_workflows / portainer_gitops_workflow -----------------

// TestGitopsWorkflowsDataSource_HealthAggregation: the three phase statuses are
// exposed separately and `healthy` is only true when all three are healthy.
func TestGitopsWorkflowsDataSource_HealthAggregation(t *testing.T) {
	t.Parallel()
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
	mock.On("GET", "/gitops/workflows/summary", RespondJSON(http.StatusOK, map[string]interface{}{"healthy": 1, "error": 1}))

	const ds = "data.portainer_gitops_workflows.w"
	gitopsDSUnitTest(t, mock, `data "portainer_gitops_workflows" "w" {
  endpoint_ids = [3, 4]
  platform     = "kubernetes"
}`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(ds, "workflows.#", "2"),
		resource.TestCheckResourceAttr(ds, "workflows.0.id", "1"),
		resource.TestCheckResourceAttr(ds, "workflows.0.healthy", "true"),
		resource.TestCheckResourceAttr(ds, "workflows.1.healthy", "false"),
		resource.TestCheckResourceAttr(ds, "workflows.1.target_error", "namespace not found"),
		resource.TestCheckResourceAttr(ds, "summary.0.drifted", "0"),
		gitopsDSCheckRequest(mock, "GET", "/gitops/workflows", func(req *RecordedRequest) error {
			if !strings.Contains(req.Query, "endpointIds=3%2C4") {
				return fmt.Errorf("endpoint_ids should be sent as a comma-separated list, got %q", req.Query)
			}
			if !strings.Contains(req.Query, "platform=kubernetes") {
				return fmt.Errorf("query should carry the platform filter, got %q", req.Query)
			}
			return nil
		}),
	))
}

// TestGitopsWorkflowsDataSource_ReconcileFilters covers what Portainer 3.0
// added to the list: the `drifted` status and `gitopsDeployment` type filters,
// and the drifted count in the summary.
func TestGitopsWorkflowsDataSource_ReconcileFilters(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/gitops/workflows", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("GET", "/gitops/workflows/summary", RespondJSON(http.StatusOK, map[string]interface{}{"healthy": 2, "drifted": 1}))

	const ds = "data.portainer_gitops_workflows.w"
	gitopsDSUnitTest(t, mock, `data "portainer_gitops_workflows" "w" {
  status = "drifted"
  type   = "gitopsDeployment"
}`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(ds, "workflows.#", "0"),
		resource.TestCheckResourceAttr(ds, "summary.0.drifted", "1"),
		resource.TestCheckResourceAttr(ds, "summary.0.healthy", "2"),
		gitopsDSCheckRequest(mock, "GET", "/gitops/workflows", func(req *RecordedRequest) error {
			if !strings.Contains(req.Query, "status=drifted") || !strings.Contains(req.Query, "type=gitopsDeployment") {
				return fmt.Errorf("query should carry both filters, got %q", req.Query)
			}
			return nil
		}),
	))
}

func TestGitopsWorkflowDataSource_Single(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/gitops/workflows/5", RespondJSON(http.StatusOK, map[string]interface{}{
		"id": 5, "name": "web", "creationDate": 1756800000, "lastSyncDate": 1756900000,
		"status": map[string]interface{}{
			"source":   map[string]interface{}{"status": "healthy"},
			"artifact": map[string]interface{}{"status": "syncing"},
			"target":   map[string]interface{}{"status": "healthy"},
		},
	}))

	const ds = "data.portainer_gitops_workflow.w"
	gitopsDSUnitTest(t, mock, `data "portainer_gitops_workflow" "w" { workflow_id = 5 }`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(ds, "id", "5"),
		resource.TestCheckResourceAttr(ds, "name", "web"),
		resource.TestCheckResourceAttr(ds, "artifact_status", "syncing"),
		resource.TestCheckResourceAttr(ds, "healthy", "false"),
		resource.TestCheckResourceAttr(ds, "creation_date", "1756800000"),
	))
}

// TestGitopsWorkflowDataSource_SchemaMatchesList keeps portainer_gitops_workflow
// and the elements of portainer_gitops_workflows in step: both are filled from
// gitopsWorkflow.attrValues().
func TestGitopsWorkflowDataSource_SchemaMatchesList(t *testing.T) {
	t.Parallel()
	var resp datasource.SchemaResponse
	newGitopsWorkflowDataSource().Schema(context.Background(), datasource.SchemaRequest{}, &resp)
	single := resp.Schema.Attributes

	for name := range gitopsWorkflowAttrTypes {
		if name == "id" {
			continue
		}
		if _, ok := single[name]; !ok {
			t.Errorf("portainer_gitops_workflow is missing %q, which portainer_gitops_workflows exposes", name)
		}
	}
	for name := range single {
		if name == "workflow_id" || name == "id" {
			continue
		}
		if _, ok := gitopsWorkflowAttrTypes[name]; !ok {
			t.Errorf("portainer_gitops_workflow exposes %q, which portainer_gitops_workflows does not", name)
		}
	}
}

// --- portainer_gitops_workflow_deployment_resources -------------------------

func TestGitopsWorkflowDeploymentResourcesDataSource_Read(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/gitops/workflows/4/deployments/21/resources", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"apiVersion": "apps/v1", "kind": "Deployment", "name": "web", "namespace": "web"},
		{"apiVersion": "v1", "kind": "Namespace", "name": "web"},
	}))

	const ds = "data.portainer_gitops_workflow_deployment_resources.r"
	gitopsDSUnitTest(t, mock, `data "portainer_gitops_workflow_deployment_resources" "r" {
  workflow_id   = 4
  deployment_id = 21
}`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(ds, "id", "4:21"),
		resource.TestCheckResourceAttr(ds, "resources.#", "2"),
		resource.TestCheckResourceAttr(ds, "resources.0.api_version", "apps/v1"),
		resource.TestCheckResourceAttr(ds, "resources.0.kind", "Deployment"),
		resource.TestCheckResourceAttr(ds, "resources.0.name", "web"),
		resource.TestCheckResourceAttr(ds, "resources.0.namespace", "web"),
		resource.TestCheckResourceAttr(ds, "resources.1.namespace", ""),
	))
}

// TestGitopsWorkflowDeploymentResourcesDataSource_OlderServer: the route is new
// in 3.0, so a 2.45 server's 404 names the requirement.
func TestGitopsWorkflowDeploymentResourcesDataSource_OlderServer(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/gitops/workflows/4/deployments/21/resources", RespondString(http.StatusNotFound, "text/plain", "404 page not found"))
	mock.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{
		"ServerVersion": "2.45.3", "ServerEdition": "BE",
	}))

	gitopsDSUnitTestError(t, mock, `data "portainer_gitops_workflow_deployment_resources" "r" {
  workflow_id   = 4
  deployment_id = 21
}`, regexp.MustCompile(`requires Portainer Business Edition 3.0 or later`))
}

// --- portainer_gitops_source_connection -------------------------------------

// TestGitopsSourceConnectionDataSource_AdHoc: the ad-hoc test posts the URL to
// the unstored endpoint.
func TestGitopsSourceConnectionDataSource_AdHoc(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/gitops/sources/test", RespondJSON(http.StatusOK, map[string]interface{}{"success": true}))

	const ds = "data.portainer_gitops_source_connection.c"
	gitopsDSUnitTest(t, mock, `data "portainer_gitops_source_connection" "c" {
  url      = "https://github.com/acme/infra.git"
  username = "ci"
  password = "token"
}`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(ds, "success", "true"),
		resource.TestCheckResourceAttr(ds, "error", ""),
		resource.TestCheckResourceAttr(ds, "fail_on_error", "false"),
		resource.TestCheckResourceAttr(ds, "id", "gitops-source-test-https://github.com/acme/infra.git"),
		gitopsDSCheckRequest(mock, "POST", "/gitops/sources/test", func(req *RecordedRequest) error {
			payload, err := gitopsDSPayload(req)
			if err != nil {
				return err
			}
			if payload["url"] != "https://github.com/acme/infra.git" {
				return fmt.Errorf("payload.url: got %v", payload["url"])
			}
			if payload["tlsSkipVerify"] != false {
				return fmt.Errorf("payload.tlsSkipVerify: got %v", payload["tlsSkipVerify"])
			}
			auth, _ := payload["authentication"].(map[string]interface{})
			if auth["username"] != "ci" || auth["password"] != "token" {
				return fmt.Errorf("payload.authentication: got %v", payload["authentication"])
			}
			return nil
		}),
	))
}

// TestGitopsSourceConnectionDataSource_StoredSource: a source_id routes to the
// stored-source endpoint, the URL is not sent there, and a failed connection
// is reported rather than raised.
func TestGitopsSourceConnectionDataSource_StoredSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/gitops/sources/12/test", RespondJSON(http.StatusOK, map[string]interface{}{
		"success": false, "error": "authentication failed",
	}))

	const ds = "data.portainer_gitops_source_connection.c"
	gitopsDSUnitTest(t, mock, `data "portainer_gitops_source_connection" "c" { source_id = 12 }`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(ds, "success", "false"),
		resource.TestCheckResourceAttr(ds, "error", "authentication failed"),
		resource.TestCheckResourceAttr(ds, "id", "gitops-source-test-12"),
		gitopsDSCheckRequest(mock, "POST", "/gitops/sources/12/test", func(req *RecordedRequest) error {
			payload, err := gitopsDSPayload(req)
			if err != nil {
				return err
			}
			for _, key := range []string{"url", "authentication"} {
				if _, ok := payload[key]; ok {
					return fmt.Errorf("%s must not be sent, got %v", key, payload[key])
				}
			}
			return nil
		}),
	))
}

// TestGitopsSourceConnectionDataSource_FailOnError: the opt-in failure mode,
// and neither-or-both inputs are rejected.
func TestGitopsSourceConnectionDataSource_FailOnError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/gitops/sources/12/test", RespondJSON(http.StatusOK, map[string]interface{}{
		"success": false, "error": "authentication failed",
	}))

	gitopsDSUnitTestError(t, mock, `data "portainer_gitops_source_connection" "c" {
  source_id     = 12
  fail_on_error = true
}`, regexp.MustCompile(`the GitOps source connection test failed: authentication failed`))

	gitopsDSUnitTestError(t, mock, `data "portainer_gitops_source_connection" "c" {}`,
		regexp.MustCompile(`exactly one of source_id or url must be set`))

	gitopsDSUnitTestError(t, mock, `data "portainer_gitops_source_connection" "c" {
  source_id = 12
  url       = "https://github.com/acme/infra.git"
}`, regexp.MustCompile(`cannot be specified when`))
}

// --- portainer_gitops_repo_file_search / portainer_gitops_helm_values -------

// TestGitopsRepoFileSearchDataSource_OmitsUnsetCredentials keeps a public
// repository from being cloned with a blank username and password.
func TestGitopsRepoFileSearchDataSource_OmitsUnsetCredentials(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/gitops/repo/files/search", RespondJSON(http.StatusOK, []string{"b/stack.yaml", "a/stack.yaml"}))

	const ds = "data.portainer_gitops_repo_file_search.s"
	gitopsDSUnitTest(t, mock, `data "portainer_gitops_repo_file_search" "s" {
  repository = "https://github.com/example/platform"
  reference  = "refs/heads/main"
  include    = "yml,yaml"
  force      = true
}`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(ds, "paths.#", "2"),
		resource.TestCheckResourceAttr(ds, "paths.0", "a/stack.yaml"),
		resource.TestCheckResourceAttr(ds, "directories_only", "false"),
		resource.TestCheckResourceAttr(ds, "tls_skip_verify", "false"),
		resource.TestCheckResourceAttr(ds, "id", "portainer-gitops-repo-file-search-https://github.com/example/platform"),
		gitopsDSCheckRequest(mock, "POST", "/gitops/repo/files/search", func(req *RecordedRequest) error {
			if !strings.Contains(req.Query, "force=true") {
				return fmt.Errorf("expected the force flag in the query, got %q", req.Query)
			}
			payload, err := gitopsDSPayload(req)
			if err != nil {
				return err
			}
			for _, key := range []string{"username", "password", "sourceId", "keyword"} {
				if _, ok := payload[key]; ok {
					return fmt.Errorf("%s must be omitted when unset, got %v", key, payload[key])
				}
			}
			if payload["include"] != "yml,yaml" || payload["dirOnly"] != false || payload["TLSSkipVerify"] != false {
				return fmt.Errorf("unexpected payload: %v", payload)
			}
			return nil
		}),
	))
}

// TestGitopsHelmValuesDataSource_KeepsFileOrder: the order the files were
// merged in decides which value wins, so it is never sorted.
func TestGitopsHelmValuesDataSource_KeepsFileOrder(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/gitops/repo/helm/values", RespondJSON(http.StatusOK, map[string]interface{}{
		"mergedValues":   "replicas: 3\n",
		"filesProcessed": []string{"values.yaml", "values-prod.yaml"},
		"commitHash":     "abc123",
	}))

	const ds = "data.portainer_gitops_helm_values.v"
	gitopsDSUnitTest(t, mock, `data "portainer_gitops_helm_values" "v" {
  repository   = "https://github.com/example/platform"
  reference    = "refs/heads/main"
  values_files = ["values.yaml", "values-prod.yaml"]
  source_id    = 3
}`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(ds, "files_processed.#", "2"),
		resource.TestCheckResourceAttr(ds, "files_processed.0", "values.yaml"),
		resource.TestCheckResourceAttr(ds, "files_processed.1", "values-prod.yaml"),
		resource.TestCheckResourceAttr(ds, "commit_hash", "abc123"),
		resource.TestCheckResourceAttr(ds, "merged_values", "replicas: 3\n"),
		gitopsDSCheckRequest(mock, "POST", "/gitops/repo/helm/values", func(req *RecordedRequest) error {
			var payload struct {
				ValuesFiles []string `json:"valuesFiles"`
				SourceID    int      `json:"sourceId"`
			}
			if err := json.Unmarshal(req.Body, &payload); err != nil {
				return err
			}
			if len(payload.ValuesFiles) != 2 || payload.ValuesFiles[0] != "values.yaml" {
				return fmt.Errorf("the values files must be sent in the configured order: %v", payload.ValuesFiles)
			}
			if payload.SourceID != 3 {
				return fmt.Errorf("sourceId: got %d", payload.SourceID)
			}
			return nil
		}),
	))
}
