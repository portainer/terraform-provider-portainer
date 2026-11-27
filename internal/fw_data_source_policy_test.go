package internal

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func policyDSUnitTest(t *testing.T, mock *MockServer, config string, check resource.TestCheckFunc) {
	t.Helper()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps:                    []resource.TestStep{{Config: mockProviderConfig(mock) + config, Check: check}},
	})
}

func policyDSExpectError(t *testing.T, mock *MockServer, config string, re string) {
	t.Helper()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps:                    []resource.TestStep{{Config: mockProviderConfig(mock) + config, ExpectError: regexp.MustCompile(re)}},
	})
}

// TestPolicyDataSource_ByID: a direct GET when policy_id is set.
func TestPolicyDataSource_ByID(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/policies/42", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 42, "Name": "Block root", "EnvironmentType": "docker", "Type": "security",
		"EnvironmentGroups": []int{1, 2},
		"Data":              map[string]interface{}{"some": "config"},
		"CreatedAt":         "2024-01-01T00:00:00Z",
		"UpdatedAt":         "2024-01-02T00:00:00Z",
	}))
	policyDSUnitTest(t, mock, `data "portainer_policy" "p" { policy_id = 42 }`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr("data.portainer_policy.p", "id", "42"),
		resource.TestCheckResourceAttr("data.portainer_policy.p", "name", "Block root"),
		resource.TestCheckResourceAttr("data.portainer_policy.p", "environment_type", "docker"),
		resource.TestCheckResourceAttr("data.portainer_policy.p", "policy_type", "security"),
		resource.TestCheckResourceAttr("data.portainer_policy.p", "created_at", "2024-01-01T00:00:00Z"),
		resource.TestCheckResourceAttr("data.portainer_policy.p", "updated_at", "2024-01-02T00:00:00Z"),
		resource.TestCheckResourceAttr("data.portainer_policy.p", "environment_groups.#", "2"),
		resource.TestCheckResourceAttr("data.portainer_policy.p", "data", `{"some":"config"}`),
	))
}

// TestPolicyDataSource_ByName: the list+filter+detail chain.
func TestPolicyDataSource_ByName(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/policies", RespondJSON(http.StatusOK, map[string]interface{}{
		"policies": []map[string]interface{}{{"Id": 7, "Name": "Block root"}, {"Id": 8, "Name": "Allow registry"}},
	}))
	mock.On("GET", "/policies/8", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 8, "Name": "Allow registry", "EnvironmentType": "docker", "Type": "registry",
	}))
	policyDSUnitTest(t, mock, `data "portainer_policy" "p" { name = "Allow registry" }`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr("data.portainer_policy.p", "id", "8"),
		resource.TestCheckResourceAttr("data.portainer_policy.p", "policy_type", "registry"),
		resource.TestCheckNoResourceAttr("data.portainer_policy.p", "data"),
		resource.TestCheckNoResourceAttr("data.portainer_policy.p", "policy_id"),
	))
}

func TestPolicyDataSource_NameNotFound(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/policies", RespondJSON(http.StatusOK, map[string]interface{}{
		"policies": []map[string]interface{}{{"Id": 1, "Name": "Other"}},
	}))
	policyDSExpectError(t, mock, `data "portainer_policy" "p" { name = "ghost" }`, `policy with name "ghost" not found`)
}

func TestPolicyDataSource_HTTPError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/policies/42", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))
	policyDSExpectError(t, mock, `data "portainer_policy" "p" { policy_id = 42 }`, `failed to read policy 42`)
}

func TestPolicyDataSource_ExactlyOneOf(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	policyDSExpectError(t, mock, `data "portainer_policy" "p" {}`, `Exactly one of these attributes must be configured`)
	policyDSExpectError(t, mock, `data "portainer_policy" "p" {
  policy_id = 1
  name      = "x"
}`, `Exactly one of these attributes must be configured`)
}

// TestPolicyTemplateDataSource_ByID: the direct GET branch.
func TestPolicyTemplateDataSource_ByID(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/policies/templates/no-root-containers", RespondJSON(http.StatusOK, map[string]interface{}{
		"id": "no-root-containers", "name": "Block root containers", "description": "Disallow root user in containers",
		"category": "security", "type": "security", "data": map[string]interface{}{"rule": "block-root"},
	}))
	policyDSUnitTest(t, mock, `data "portainer_policy_template" "t" { template_id = "no-root-containers" }`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr("data.portainer_policy_template.t", "id", "no-root-containers"),
		resource.TestCheckResourceAttr("data.portainer_policy_template.t", "name", "Block root containers"),
		resource.TestCheckResourceAttr("data.portainer_policy_template.t", "description", "Disallow root user in containers"),
		resource.TestCheckResourceAttr("data.portainer_policy_template.t", "category", "security"),
		resource.TestCheckResourceAttr("data.portainer_policy_template.t", "policy_type", "security"),
		resource.TestCheckResourceAttr("data.portainer_policy_template.t", "data", `{"rule":"block-root"}`),
	))
}

func TestPolicyTemplateDataSource_ByName(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/policies/templates", RespondJSON(http.StatusOK, map[string]interface{}{
		"templates": []map[string]interface{}{{"id": "rbac-admin", "name": "RBAC admin"}, {"id": "no-root", "name": "Block root"}},
	}))
	mock.On("GET", "/policies/templates/no-root", RespondJSON(http.StatusOK, map[string]interface{}{
		"id": "no-root", "name": "Block root", "category": "security", "type": "security",
	}))
	policyDSUnitTest(t, mock, `data "portainer_policy_template" "t" { name = "Block root" }`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr("data.portainer_policy_template.t", "id", "no-root"),
		resource.TestCheckResourceAttr("data.portainer_policy_template.t", "category", "security"),
	))
}

func TestPolicyTemplateDataSource_NameNotFound(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/policies/templates", RespondJSON(http.StatusOK, map[string]interface{}{
		"templates": []map[string]interface{}{{"id": "rbac-admin", "name": "RBAC admin"}},
	}))
	policyDSExpectError(t, mock, `data "portainer_policy_template" "t" { name = "ghost" }`, `policy template with name "ghost" not found`)
}

func TestPolicyTemplateDataSource_HTTPError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/policies/templates/missing", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))
	policyDSExpectError(t, mock, `data "portainer_policy_template" "t" { template_id = "missing" }`, `failed to read policy template missing`)
}

func TestPolicyMetadataDataSource_Reads(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/policies/metadata", RespondJSON(http.StatusOK, map[string]interface{}{
		"minimumAgentVersions": map[string]string{"rbac-k8s": "2.20.0", "observability-k8s": "2.27.0"},
	}))
	policyDSUnitTest(t, mock, `data "portainer_policy_metadata" "m" {}`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr("data.portainer_policy_metadata.m", "id", "portainer-policy-metadata"),
		resource.TestCheckResourceAttr("data.portainer_policy_metadata.m", "minimum_agent_versions.%", "2"),
		resource.TestCheckResourceAttr("data.portainer_policy_metadata.m", "minimum_agent_versions.rbac-k8s", "2.20.0"),
	))
}

// TestPolicyConflictsDataSource_PassesPolicyThrough pins the JSON
// passthrough, which is what Portainer's untyped payload forces.
func TestPolicyConflictsDataSource_PassesPolicyThrough(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/policies/conflicts", RespondJSON(http.StatusOK, map[string]interface{}{
		"totalEnvironments": 10, "supportedEnvironments": 8, "unsupportedEnvironments": 2,
		"conflicts": []map[string]interface{}{{
			"environmentGroupId": 3, "environmentGroupName": "shops",
			"environmentCount": 5, "supportedEnvironments": 4, "unsupportedEnvironments": 1,
			"existingPolicyId": 7, "existingPolicyName": "baseline",
		}},
		"newGroups": []map[string]interface{}{{
			"environmentGroupId": 4, "environmentGroupName": "warehouses",
			"environmentCount": 5, "supportedEnvironments": 4, "unsupportedEnvironments": 1,
		}},
	}))
	const ds = "data.portainer_policy_conflicts.c"
	policyDSUnitTest(t, mock, `data "portainer_policy_conflicts" "c" {
  policy_json = jsonencode({ name = "baseline-v2", environmentGroupIds = [3, 4] })
}`, resource.ComposeTestCheckFunc(
		func(*terraform.State) error {
			var payload map[string]interface{}
			if err := mock.FindRequest("POST", "/policies/conflicts").DecodeJSON(&payload); err != nil {
				return fmt.Errorf("payload is not JSON: %w", err)
			}
			if payload["name"] != "baseline-v2" {
				return fmt.Errorf("the candidate policy must be sent as given, got %v", payload)
			}
			return nil
		},
		resource.TestCheckResourceAttr(ds, "id", "portainer-policy-conflicts"),
		resource.TestCheckResourceAttr(ds, "total_environments", "10"),
		resource.TestCheckResourceAttr(ds, "unsupported_environments", "2"),
		resource.TestCheckResourceAttr(ds, "conflicts.#", "1"),
		resource.TestCheckResourceAttr(ds, "conflicts.0.existing_policy_name", "baseline"),
		resource.TestCheckResourceAttr(ds, "conflicts.0.existing_policy_id", "7"),
		resource.TestCheckResourceAttr(ds, "new_groups.#", "1"),
		resource.TestCheckResourceAttr(ds, "new_groups.0.environment_group_name", "warehouses"),
	))
}

func TestPolicyConflictsDataSource_RejectsNonObject(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	policyDSExpectError(t, mock, `data "portainer_policy_conflicts" "c" { policy_json = "[1,2]" }`, `must be a JSON object`)
}

// TestPolicyObservabilityTestDataSource_OmitsUnsetCredentials keeps the test
// from sending a blank key OneUptime would reject for the wrong reason, and
// sends the SDK defaults for the flags left out.
func TestPolicyObservabilityTestDataSource_OmitsUnsetCredentials(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/policies/observability-k8s/test", RespondJSON(http.StatusOK, map[string]interface{}{
		"success": true, "message": "connected",
	}))
	const ds = "data.portainer_policy_observability_test.o"
	policyDSUnitTest(t, mock, `data "portainer_policy_observability_test" "o" { one_uptime_url = "https://oneuptime.example.com" }`,
		resource.ComposeTestCheckFunc(
			func(*terraform.State) error {
				var payload map[string]interface{}
				if err := mock.FindRequest("POST", "/policies/observability-k8s/test").DecodeJSON(&payload); err != nil {
					return fmt.Errorf("payload is not JSON: %w", err)
				}
				for _, key := range []string{"apiKey", "projectID", "policyId"} {
					if _, ok := payload[key]; ok {
						return fmt.Errorf("%s must be omitted when unset, got %v", key, payload[key])
					}
				}
				for _, key := range []string{"tlsSkipVerify", "autoProvisionIngestionKey", "autoRotateKey"} {
					if payload[key] != false {
						return fmt.Errorf("%s: want false, got %v", key, payload[key])
					}
				}
				return nil
			},
			resource.TestCheckResourceAttr(ds, "success", "true"),
			resource.TestCheckResourceAttr(ds, "message", "connected"),
			resource.TestCheckResourceAttr(ds, "id", "portainer-policy-observability-test"),
		))
}

func TestPolicyObservabilityTestDataSource_SendsCredentials(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/policies/observability-k8s/test", RespondJSON(http.StatusOK, map[string]interface{}{"success": true}))
	policyDSUnitTest(t, mock, `data "portainer_policy_observability_test" "o" {
  one_uptime_url  = "https://oneuptime.example.com"
  api_key         = "k"
  project_id      = "proj"
  policy_id       = 3
  tls_skip_verify = true
}`, func(*terraform.State) error {
		var payload map[string]interface{}
		if err := mock.FindRequest("POST", "/policies/observability-k8s/test").DecodeJSON(&payload); err != nil {
			return err
		}
		if payload["apiKey"] != "k" || payload["projectID"] != "proj" || payload["policyId"] != float64(3) || payload["tlsSkipVerify"] != true {
			return fmt.Errorf("unexpected payload %v", payload)
		}
		return nil
	})
}

// TestPolicyObservabilityTestDataSource_FailsByDefault covers the
// pre-flight default: a failed connection stops the plan.
func TestPolicyObservabilityTestDataSource_FailsByDefault(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/policies/observability-k8s/test", RespondJSON(http.StatusOK, map[string]interface{}{
		"success": false, "message": "project not found",
	}))
	policyDSExpectError(t, mock, `data "portainer_policy_observability_test" "o" { one_uptime_url = "https://oneuptime.example.com" }`, `project not found`)
}

func TestPolicyObservabilityTestDataSource_ReportsWhenAsked(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/policies/observability-k8s/test", RespondJSON(http.StatusOK, map[string]interface{}{
		"success": false, "message": "project not found",
	}))
	policyDSUnitTest(t, mock, `data "portainer_policy_observability_test" "o" {
  one_uptime_url = "https://oneuptime.example.com"
  fail_on_error  = false
}`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr("data.portainer_policy_observability_test.o", "success", "false"),
		resource.TestCheckResourceAttr("data.portainer_policy_observability_test.o", "message", "project not found"),
	))
}

// TestAccPolicyDataSources_basic reads the policy templates and metadata of
// a Business Edition instance.
func TestAccPolicyDataSources_basic(t *testing.T) {
	if os.Getenv("PORTAINER_ACC_BE") != "1" {
		t.Skip("policy data sources need Portainer Business Edition; set PORTAINER_ACC_BE=1")
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: accProvider() + `data "portainer_policy_metadata" "m" {}`,
			Check:  resource.TestCheckResourceAttr("data.portainer_policy_metadata.m", "id", "portainer-policy-metadata"),
		}},
	})
}
