package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// actionConfig invokes the action typ with the given config body once, from
// the after_create trigger of a terraform_data resource.
func actionConfig(mock *MockServer, typ, body string) string {
	return mockProviderConfig(mock) + fmt.Sprintf(`
action %[1]q "this" {
  config {
%[2]s
  }
}

resource "terraform_data" "trigger" {
  input = "go"
  lifecycle {
    action_trigger {
      events  = [after_create]
      actions = [action.%[1]s.this]
    }
  }
}
`, typ, body)
}

// testAction applies config and runs check against the requests the mock
// received.
func testAction(t *testing.T, config string, check resource.TestCheckFunc) {
	t.Helper()
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_14_0), skipOnOpenTofuActions},
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps:                    []resource.TestStep{{Config: config, Check: check}},
	})
}

// expectRequests checks that the mock received exactly these requests
// ("METHOD /path"), in order.
func expectRequests(mock *MockServer, want ...string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		reqs := mock.Requests()
		got := make([]string, 0, len(reqs))
		for _, r := range reqs {
			got = append(got, r.Method+" "+r.Path)
		}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			return fmt.Errorf("requests: got %q, want %q", got, want)
		}
		return nil
	}
}

// findRequest returns the recorded request "METHOD /path".
func findRequest(mock *MockServer, req string) (*RecordedRequest, error) {
	method, p, _ := strings.Cut(req, " ")
	if r := mock.FindRequest(method, p); r != nil {
		return r, nil
	}
	return nil, fmt.Errorf("no request %s", req)
}

// expectBody checks that the JSON body of request req has the fields of want.
func expectBody(mock *MockServer, req, want string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		r, err := findRequest(mock, req)
		if err != nil {
			return err
		}
		var got, exp map[string]any
		if err := r.DecodeJSON(&got); err != nil {
			return fmt.Errorf("%s: body %q: %w", req, r.Body, err)
		}
		if err := json.Unmarshal([]byte(want), &exp); err != nil {
			return err
		}
		for k, v := range exp {
			if !reflect.DeepEqual(got[k], v) {
				return fmt.Errorf("%s: body field %s = %#v, want %#v (body %s)", req, k, got[k], v, r.Body)
			}
		}
		return nil
	}
}

// expectQuery checks the query string of request req.
func expectQuery(mock *MockServer, req, want string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		r, err := findRequest(mock, req)
		if err != nil {
			return err
		}
		if r.Query != want {
			return fmt.Errorf("%s: query %q, want %q", req, r.Query, want)
		}
		return nil
	}
}

// Each action sends what its resource's Create sends: the tests below follow
// the resource tests' requests, with the schema defaults the action applies
// itself.

func TestEndpointSnapshotAction(t *testing.T) {
	t.Parallel()
	t.Run("one", func(t *testing.T) {
		mock := NewMockServer(t)
		mock.On("POST", "/endpoints/3/snapshot", RespondString(http.StatusNoContent, "", ""))
		testAction(t, actionConfig(mock, "portainer_endpoint_snapshot", `endpoint_id = 3`),
			expectRequests(mock, "POST /endpoints/3/snapshot"))
	})
	t.Run("all", func(t *testing.T) {
		mock := NewMockServer(t)
		mock.On("POST", "/endpoints/snapshot", RespondString(http.StatusNoContent, "", ""))
		testAction(t, actionConfig(mock, "portainer_endpoint_snapshot", ``),
			expectRequests(mock, "POST /endpoints/snapshot"))
	})
}

// TestEndpointSnapshotAction_Error: a failing operation fails the apply that
// triggered the action, with the resource's error.
func TestEndpointSnapshotAction_Error(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/endpoints/9/snapshot", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_14_0), skipOnOpenTofuActions},
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      actionConfig(mock, "portainer_endpoint_snapshot", `endpoint_id = 9`),
			ExpectError: regexp.MustCompile(`HTTP 500`),
		}},
	})
}

func TestBackupAction(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/backup", RespondString(http.StatusOK, "application/x-gzip", "ARCHIVE"))
	out := filepath.Join(t.TempDir(), "backup.tar.gz")
	testAction(t, actionConfig(mock, "portainer_backup", fmt.Sprintf(`output_path = %q
    password    = "s3cret"`, out)), resource.ComposeTestCheckFunc(
		expectRequests(mock, "POST /backup"),
		expectBody(mock, "POST /backup", `{"password":"s3cret"}`),
		func(_ *terraform.State) error {
			b, err := os.ReadFile(out)
			if err != nil {
				return fmt.Errorf("backup file: %w", err)
			}
			if string(b) != "ARCHIVE" {
				return fmt.Errorf("backup file content %q", b)
			}
			return nil
		},
	))
}

func TestBackupS3Action(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/backup/s3/execute", RespondJSON(http.StatusOK, map[string]any{}))
	testAction(t, actionConfig(mock, "portainer_backup_s3", `access_key_id      = "AK"
    secret_access_key  = "SK"
    bucket_name        = "b"
    region             = "eu"
    s3_compatible_host = "https://s3"
    password           = "pw"`), resource.ComposeTestCheckFunc(
		expectRequests(mock, "POST /backup/s3/execute"),
		expectBody(mock, "POST /backup/s3/execute", `{"accessKeyID":"AK","secretAccessKey":"SK","bucketName":"b","region":"eu","s3CompatibleHost":"https://s3","password":"pw"}`),
	))
}

func TestBackupLocalRunAction(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/backup/local/run", RespondString(http.StatusNoContent, "", ""))
	mock.On("GET", "/backup/local/status", RespondJSON(http.StatusOK, map[string]any{"failed": false, "timestampUTC": "now", "path": "/data/b.tar.gz"}))
	testAction(t, actionConfig(mock, "portainer_backup_local_run", ``),
		expectRequests(mock, "POST /backup/local/run", "GET /backup/local/status"))
}

func TestBackupAzureExecuteAction(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/backup/azure/execute", RespondString(http.StatusNoContent, "", ""))
	testAction(t, actionConfig(mock, "portainer_backup_azure_execute", `auth_method          = "storageAccountKey"
    storage_account_name = "acct"
    container_name       = "backups"
    storage_account_key  = "key"`), resource.ComposeTestCheckFunc(
		expectRequests(mock, "POST /backup/azure/execute"),
		expectBody(mock, "POST /backup/azure/execute", `{"authMethod":"storageAccountKey","storageAccountName":"acct","containerName":"backups","storageAccountKey":"key"}`),
	))
}

func TestWebhookExecuteAction(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/webhooks/tok", RespondString(http.StatusNoContent, "", ""))
	testAction(t, actionConfig(mock, "portainer_webhook_execute", `token = "tok"`), resource.ComposeTestCheckFunc(
		expectRequests(mock, "POST /webhooks/tok"),
		// Webhooks are public: like the resource, the action sends no credentials.
		func(_ *terraform.State) error {
			if h := mock.Requests()[0].Headers.Get("X-API-Key"); h != "" {
				return fmt.Errorf("webhook call carried X-API-Key %q", h)
			}
			return nil
		},
	))
}

func TestEdgeStackWebhookAction(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/edge_stacks/webhooks/uuid-1", RespondString(http.StatusNoContent, "", ""))
	testAction(t, actionConfig(mock, "portainer_edge_stack_webhook", `webhook_id = "uuid-1"`),
		expectRequests(mock, "POST /edge_stacks/webhooks/uuid-1"))
}

func TestContainerExecAction(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/1/docker/containers/json", RespondJSON(http.StatusOK, []map[string]any{{"Id": "c1"}}))
	mock.On("POST", "/endpoints/1/docker/containers/c1/exec", RespondJSON(http.StatusCreated, map[string]any{"Id": "x1"}))
	mock.On("POST", "/endpoints/1/docker/exec/x1/start", RespondString(http.StatusOK, "text/plain", "hello"))
	testAction(t, actionConfig(mock, "portainer_container_exec", `endpoint_id  = 1
    service_name = "web"
    command      = "echo hello"`), resource.ComposeTestCheckFunc(
		expectRequests(mock, "GET /endpoints/1/docker/containers/json", "POST /endpoints/1/docker/containers/c1/exec", "POST /endpoints/1/docker/exec/x1/start"),
		// user is the resource's default.
		expectBody(mock, "POST /endpoints/1/docker/containers/c1/exec", `{"User":"root:root","Cmd":["echo","hello"]}`),
	))
}

func TestDeployAction(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/1/docker/swarm", RespondString(http.StatusServiceUnavailable, "application/json", `{"message":"not a swarm"}`))
	mock.On("GET", "/stacks", RespondJSON(http.StatusOK, []map[string]any{{"Id": 4, "Name": "web"}}))
	testAction(t, actionConfig(mock, "portainer_deploy", `endpoint_id     = 1
    stack_name      = "web"
    services_list   = "app"
    revision        = "1.2.3"
    stack_env_var   = "APP_VERSION"
    update_revision = false`),
		expectRequests(mock, "GET /endpoints/1/docker/swarm", "GET /stacks"))
}

func TestCheckAction(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/1/docker/swarm", RespondString(http.StatusServiceUnavailable, "application/json", `{"message":"not a swarm"}`))
	mock.On("GET", "/endpoints/1/docker/containers/json", RespondJSON(http.StatusOK, []map[string]any{
		{"Names": []string{"/web_app_1"}, "State": "running", "Image": "nginx:1.2"},
	}))
	testAction(t, actionConfig(mock, "portainer_check", `endpoint_id   = 1
    stack_name    = "web"
    services_list = "app"
    revision      = "1.2"
    wait          = 0`), resource.ComposeTestCheckFunc(
		expectRequests(mock, "GET /endpoints/1/docker/swarm", "GET /endpoints/1/docker/containers/json"),
		expectQuery(mock, "GET /endpoints/1/docker/containers/json", "all=1"),
	))
}

// TestCheckAction_Fails: a service that never matches fails the apply.
func TestCheckAction_Fails(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/1/docker/swarm", RespondString(http.StatusServiceUnavailable, "application/json", `{}`))
	mock.On("GET", "/endpoints/1/docker/containers/json", RespondJSON(http.StatusOK, []map[string]any{}))
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_14_0), skipOnOpenTofuActions},
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: actionConfig(mock, "portainer_check", `endpoint_id         = 1
    stack_name          = "web"
    services_list       = "app"
    revision            = "1.2"
    wait                = 0
    wait_between_checks = 0
    max_retries         = 1`),
			ExpectError: regexp.MustCompile(`Check failed`),
		}},
	})
}

func TestEndpointServiceUpdateAction(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/1/docker/services", RespondJSON(http.StatusOK, []map[string]any{{"ID": "s1", "Spec": map[string]any{"Name": "web"}}}))
	mock.On("PUT", "/endpoints/1/forceupdateservice", RespondJSON(http.StatusOK, map[string]any{}))
	testAction(t, actionConfig(mock, "portainer_endpoint_service_update", `endpoint_id  = 1
    service_name = "web"`), resource.ComposeTestCheckFunc(
		expectRequests(mock, "GET /endpoints/1/docker/services", "PUT /endpoints/1/forceupdateservice"),
		expectBody(mock, "PUT /endpoints/1/forceupdateservice", `{"serviceID":"s1","pullImage":false}`),
	))
}

func TestEndpointAssociationAction(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("PUT", "/endpoints/5/association", RespondJSON(http.StatusOK, map[string]any{}))
	testAction(t, actionConfig(mock, "portainer_endpoint_association", `endpoint_id = 5`),
		expectRequests(mock, "PUT /endpoints/5/association"))
}

func TestKubernetesNodeDrainAction(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/kubernetes/1/nodes/n1/drain", RespondString(http.StatusNoContent, "", ""))
	testAction(t, actionConfig(mock, "portainer_kubernetes_node_drain", `environment_id = 1
    node_name      = "n1"
    force          = true`), resource.ComposeTestCheckFunc(
		expectRequests(mock, "POST /kubernetes/1/nodes/n1/drain"),
		// The unset options are sent with the resource's defaults.
		expectBody(mock, "POST /kubernetes/1/nodes/n1/drain", `{"Force":true,"TimeoutSeconds":60,"GracePeriodSeconds":-1,"IgnoreDaemonSets":true,"DeleteEmptyDirData":true,"DisableEviction":false}`),
	))
}

func TestKubernetesDeploymentScaleAction(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("PUT", "/kubernetes/1/namespaces/default/deployments/web/scale", RespondJSON(http.StatusOK, map[string]any{}))
	testAction(t, actionConfig(mock, "portainer_kubernetes_deployment_scale", `environment_id  = 1
    namespace       = "default"
    deployment_name = "web"
    replicas        = 3`), resource.ComposeTestCheckFunc(
		expectRequests(mock, "PUT /kubernetes/1/namespaces/default/deployments/web/scale"),
		expectBody(mock, "PUT /kubernetes/1/namespaces/default/deployments/web/scale", `{"replicas":3}`),
	))
}

func TestKubernetesDeploymentRollbackAction(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/kubernetes/1/namespaces/default/deployments/web/rollback", RespondJSON(http.StatusOK, map[string]any{}))
	testAction(t, actionConfig(mock, "portainer_kubernetes_deployment_rollback", `environment_id  = 1
    namespace       = "default"
    deployment_name = "web"`), resource.ComposeTestCheckFunc(
		expectRequests(mock, "POST /kubernetes/1/namespaces/default/deployments/web/rollback"),
		expectBody(mock, "POST /kubernetes/1/namespaces/default/deployments/web/rollback", `{"revision":0}`),
	))
}

func TestKubernetesClusterUpgradeAction(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/cloud/endpoints/2/upgrade", RespondJSON(http.StatusOK, map[string]any{}))
	testAction(t, actionConfig(mock, "portainer_kubernetes_cluster_upgrade", `endpoint_id = 2`),
		expectRequests(mock, "POST /cloud/endpoints/2/upgrade"))
}

func TestKubernetesDeleteObjectAction(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/kubernetes/1/services/delete", RespondString(http.StatusNoContent, "", ""))
	testAction(t, actionConfig(mock, "portainer_kubernetes_delete_object", `environment_id = 1
    resource_type  = "services"
    namespace      = "default"
    names          = ["a", "b"]`), resource.ComposeTestCheckFunc(
		expectRequests(mock, "POST /kubernetes/1/services/delete"),
		expectBody(mock, "POST /kubernetes/1/services/delete", `{"default":["a","b"]}`),
	))
}

func TestStackMigrateAction(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/stacks/7/migrate", RespondJSON(http.StatusOK, map[string]any{}))
	testAction(t, actionConfig(mock, "portainer_stack_migrate", `stack_id           = 7
    target_endpoint_id = 2
    endpoint_id        = 1`), resource.ComposeTestCheckFunc(
		expectRequests(mock, "POST /stacks/7/migrate"),
		expectQuery(mock, "POST /stacks/7/migrate", "endpointId=1"),
		expectBody(mock, "POST /stacks/7/migrate", `{"EndpointID":2}`),
	))
}

func TestStackAssociateAction(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("PUT", "/stacks/7/associate", RespondJSON(http.StatusOK, map[string]any{}))
	testAction(t, actionConfig(mock, "portainer_stack_associate", `stack_id    = 7
    endpoint_id = 2
    swarm_id    = "sw"`), resource.ComposeTestCheckFunc(
		expectRequests(mock, "PUT /stacks/7/associate"),
		expectQuery(mock, "PUT /stacks/7/associate", "endpointId=2&orphanedRunning=false&swarmId=sw"),
	))
}

func TestStackDeleteByNameAction(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	// Gone already: not an error, as for the resource.
	mock.On("DELETE", "/stacks/name/legacy", RespondJSON(http.StatusNotFound, map[string]any{"message": "not found"}))
	testAction(t, actionConfig(mock, "portainer_stack_delete_by_name", `endpoint_id = 1
    name        = "legacy"
    namespace   = "apps"`), resource.ComposeTestCheckFunc(
		expectRequests(mock, "DELETE /stacks/name/legacy"),
		expectQuery(mock, "DELETE /stacks/name/legacy", "endpointId=1&namespace=apps"),
	))
}

func TestAddonRepairAction(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/addons/portal-template/repair", RespondJSON(http.StatusOK, map[string]any{"id": "portal-template", "enabled": true}))
	testAction(t, actionConfig(mock, "portainer_addon_repair", `addon_id = "portal-template"`),
		expectRequests(mock, "POST /addons/portal-template/repair"))
}

func TestAddonCatalogRefreshAction(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/addons/catalog/refresh", RespondString(http.StatusNoContent, "", ""))
	testAction(t, actionConfig(mock, "portainer_addon_catalog_refresh", ``),
		expectRequests(mock, "POST /addons/catalog/refresh"))
}

func TestGitopsWorkflowReconcileAction(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/gitops/workflows/4/reconcile", RespondString(http.StatusNoContent, "", ""))
	testAction(t, actionConfig(mock, "portainer_gitops_workflow_reconcile", `workflow_id = 4`),
		expectRequests(mock, "POST /gitops/workflows/4/reconcile"))
}

func TestOmniNodeRebootAction(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/omni/3/node/n1", RespondString(http.StatusNoContent, "", ""))
	testAction(t, actionConfig(mock, "portainer_omni_node_reboot", `credential_id = 3
    cluster       = "c1"
    node          = "n1"`), resource.ComposeTestCheckFunc(
		expectRequests(mock, "POST /omni/3/node/n1"),
		expectQuery(mock, "POST /omni/3/node/n1", "cluster=c1"),
	))
}

func TestHelmRollbackAction(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/endpoints/1/kubernetes/helm/web/rollback", RespondJSON(http.StatusOK, map[string]any{}))
	testAction(t, actionConfig(mock, "portainer_helm_rollback", `endpoint_id  = 1
    release_name = "web"
    namespace    = "apps"`), resource.ComposeTestCheckFunc(
		expectRequests(mock, "POST /endpoints/1/kubernetes/helm/web/rollback"),
		// recreate and timeout carry the resource's defaults.
		expectQuery(mock, "POST /endpoints/1/kubernetes/helm/web/rollback", "namespace=apps&recreate=true&timeout=300"),
	))
}

func TestUserMembershipsSyncAction(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/users/5/memberships/sync", RespondString(http.StatusNoContent, "", ""))
	testAction(t, actionConfig(mock, "portainer_user_memberships_sync", `user_id = 5`),
		expectRequests(mock, "POST /users/5/memberships/sync"))
}

// TestAccEndpointSnapshotAction snapshots every environment of a real
// Portainer (CE has the endpoint) from an action trigger.
func TestAccEndpointSnapshotAction(t *testing.T) {
	accPreCheck(t)
	resource.Test(t, resource.TestCase{
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_14_0), skipOnOpenTofuActions},
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: accProvider() + `
action "portainer_endpoint_snapshot" "all" {}

resource "terraform_data" "trigger" {
  input = "acc"
  lifecycle {
    action_trigger {
      events  = [after_create]
      actions = [action.portainer_endpoint_snapshot.all]
    }
  }
}
`,
		}},
	})
}
