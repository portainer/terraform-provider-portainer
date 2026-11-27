package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// fakePolicies is a stateful stand-in for Portainer's /policies API.
type fakePolicies struct {
	mu       sync.Mutex
	nextID   int
	policies map[int]map[string]interface{}
	// zeroGroups and zeroData make GET return [] and {} for a policy
	// created without groups or data, instead of leaving the fields out.
	zeroGroups, zeroData bool
}

func newFakePolicies(m *MockServer, maxID int) *fakePolicies {
	f := &fakePolicies{policies: map[int]map[string]interface{}{}}
	m.On("POST", "/policies", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.nextID++
		body["Id"] = f.nextID
		body["CreatedAt"] = "2026-01-01T00:00:00Z"
		body["UpdatedAt"] = "2026-01-01T00:00:00Z"
		f.policies[f.nextID] = body
		RespondJSON(http.StatusOK, map[string]interface{}{"Id": f.nextID})(w, r)
	})
	for id := 1; id <= maxID; id++ {
		id := id
		p := fmt.Sprintf("/policies/%d", id)
		m.On("GET", p, func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			pol, ok := f.policies[id]
			if !ok {
				RespondString(http.StatusNotFound, "application/json", `{"message":"not found"}`)(w, r)
				return
			}
			out := map[string]interface{}{}
			for k, v := range pol {
				out[k] = v
			}
			if _, ok := out["EnvironmentGroups"]; !ok && f.zeroGroups {
				out["EnvironmentGroups"] = []int{}
			}
			if _, ok := out["Data"]; !ok && f.zeroData {
				out["Data"] = map[string]interface{}{}
			}
			RespondJSON(http.StatusOK, out)(w, r)
		})
		m.On("PUT", p, func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			defer f.mu.Unlock()
			body["Id"] = id
			body["CreatedAt"] = f.policies[id]["CreatedAt"]
			body["UpdatedAt"] = "2026-02-01T00:00:00Z"
			f.policies[id] = body
			RespondString(http.StatusOK, "application/json", `{}`)(w, r)
		})
		m.On("DELETE", p, func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			delete(f.policies, id)
			w.WriteHeader(http.StatusNoContent)
		})
	}
	return f
}

func lastPolicyPayload(t *testing.T, m *MockServer, method, p string) map[string]interface{} {
	t.Helper()
	var last *RecordedRequest
	for _, r := range m.Requests() {
		if r.Method == method && r.Path == p {
			last = r
		}
	}
	if last == nil {
		t.Fatalf("expected %s %s", method, p)
	}
	var payload map[string]interface{}
	if err := last.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	return payload
}

const policyFullConfig = `
resource "portainer_policy" "p" {
  name               = %q
  environment_type   = "kubernetes"
  policy_type        = "rbac-k8s"
  environment_groups = [1, 2]
  data               = jsonencode({ foo = "bar" })
  allow_override     = true
}
`

// TestPolicyResource_Lifecycle: create with every argument (payload with the
// PascalCase keys the API expects), an empty plan, an in-place rename (PUT),
// import and destroy.
func TestPolicyResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakePolicies(mock, 2)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(policyFullConfig, "test-policy"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_policy.p", "id", "1"),
					resource.TestCheckResourceAttr("portainer_policy.p", "name", "test-policy"),
					resource.TestCheckResourceAttr("portainer_policy.p", "created_at", "2026-01-01T00:00:00Z"),
					resource.TestCheckResourceAttr("portainer_policy.p", "environment_groups.#", "2"),
					func(_ *terraform.State) error {
						payload := lastPolicyPayload(t, mock, "POST", "/policies")
						want := map[string]interface{}{"Name": "test-policy", "EnvironmentType": "kubernetes", "Type": "rbac-k8s", "AllowOverride": true}
						for k, v := range want {
							if payload[k] != v {
								return fmt.Errorf("payload.%s: got %v, want %v", k, payload[k], v)
							}
						}
						if g, _ := payload["EnvironmentGroups"].([]interface{}); len(g) != 2 {
							return fmt.Errorf("payload.EnvironmentGroups: got %v", payload["EnvironmentGroups"])
						}
						if d, _ := payload["Data"].(map[string]interface{}); d["foo"] != "bar" {
							return fmt.Errorf("payload.Data: got %v", payload["Data"])
						}
						return nil
					},
				),
			},
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(policyFullConfig, "renamed"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_policy.p", plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_policy.p", "name", "renamed"),
					resource.TestCheckResourceAttr("portainer_policy.p", "updated_at", "2026-02-01T00:00:00Z"),
					func(_ *terraform.State) error {
						if got := lastPolicyPayload(t, mock, "PUT", "/policies/1")["Name"]; got != "renamed" {
							return fmt.Errorf("PUT payload.Name: got %v", got)
						}
						return nil
					},
				),
			},
			{
				ResourceName:      "portainer_policy.p",
				ImportState:       true,
				ImportStateVerify: true,
				// Portainer does not return allow_override; import sets the default.
				ImportStateVerifyIgnore: []string{"allow_override"},
			},
		},
	})
	if mock.FindRequest("DELETE", "/policies/1") == nil {
		t.Error("expected DELETE /policies/1")
	}
}

// TestPolicyResource_PolicyTypeReplaces: policy_type forces a new policy.
func TestPolicyResource_PolicyTypeReplaces(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakePolicies(mock, 2)
	cfg := func(pt string) string {
		return mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_policy" "p" {
  name             = "p"
  environment_type = "kubernetes"
  policy_type      = %q
}`, pt)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: cfg("rbac-k8s")},
			{
				Config: cfg("security-k8s"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_policy.p", plancheck.ResourceActionDestroyBeforeCreate)},
				},
				Check: resource.TestCheckResourceAttr("portainer_policy.p", "id", "2"),
			},
		},
	})
}

// TestPolicyResource_MinimalOmitsOptional: groups and data left out are not
// sent, stay null in state and plan no change, even when the API answers
// with an empty list and an empty object.
func TestPolicyResource_MinimalOmitsOptional(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakePolicies(mock, 1)
	fake.zeroGroups, fake.zeroData = true, true
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + `
resource "portainer_policy" "p" {
  name             = "p1"
  environment_type = "docker"
  policy_type      = "security-docker"
}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("portainer_policy.p", "environment_groups.#"),
					resource.TestCheckNoResourceAttr("portainer_policy.p", "data"),
					resource.TestCheckResourceAttr("portainer_policy.p", "allow_override", "false"),
					func(_ *terraform.State) error {
						payload := lastPolicyPayload(t, mock, "POST", "/policies")
						for _, k := range []string{"EnvironmentGroups", "Data"} {
							if _, ok := payload[k]; ok {
								return fmt.Errorf("%s must be omitted when unset, got %v", k, payload[k])
							}
						}
						if payload["AllowOverride"] != false {
							return fmt.Errorf("payload.AllowOverride: got %v", payload["AllowOverride"])
						}
						return nil
					},
				),
			},
		},
	})
}

// TestPolicyResource_VulnerabilityScanning: a 3.0 policy type and its data
// reach the API unchanged.
func TestPolicyResource_VulnerabilityScanning(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakePolicies(mock, 1)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
resource "portainer_policy" "p" {
  name             = "scan"
  environment_type = "kubernetes"
  policy_type      = "vulnerability-scanning-k8s"
  data             = jsonencode({ operatorNamespace = "trivy-system" })
}`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("portainer_policy.p", "policy_type", "vulnerability-scanning-k8s"),
				func(_ *terraform.State) error {
					payload := lastPolicyPayload(t, mock, "POST", "/policies")
					if payload["Type"] != "vulnerability-scanning-k8s" {
						return fmt.Errorf("payload.Type: got %v", payload["Type"])
					}
					if d, _ := payload["Data"].(map[string]interface{}); d["operatorNamespace"] != "trivy-system" {
						return fmt.Errorf("payload.Data: got %v", payload["Data"])
					}
					return nil
				},
			),
		}},
	})
}

// TestPolicyResource_GoneOutsideTerraform: a policy deleted in Portainer
// (GET 404) is planned for creation again.
func TestPolicyResource_GoneOutsideTerraform(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakePolicies(mock, 1)
	cfg := mockProviderConfig(mock) + `
resource "portainer_policy" "p" {
  name             = "p1"
  environment_type = "docker"
  policy_type      = "security-docker"
}`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{
				PreConfig: func() {
					fake.mu.Lock()
					delete(fake.policies, 1)
					fake.mu.Unlock()
				},
				Config:             cfg,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestPolicyResource_DriftDetected: a change made in Portainer shows up as a
// planned update.
func TestPolicyResource_DriftDetected(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakePolicies(mock, 1)
	cfg := mockProviderConfig(mock) + fmt.Sprintf(policyFullConfig, "p1")
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{
				PreConfig: func() {
					fake.mu.Lock()
					fake.policies[1]["Name"] = "changed"
					fake.policies[1]["Data"] = map[string]interface{}{"foo": "baz"}
					fake.mu.Unlock()
				},
				Config:             cfg,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestPolicyResource_CreateHTTPError: an API error fails the apply and
// leaves nothing in state.
func TestPolicyResource_CreateHTTPError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/policies", RespondString(http.StatusBadRequest, "application/json", `{"message":"bad"}`))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
resource "portainer_policy" "p" {
  name             = "bad"
  environment_type = "kubernetes"
  policy_type      = "rbac-k8s"
}`,
			ExpectError: regexp.MustCompile(`failed to create policy`),
		}},
	})
}

// TestPolicyResource_Validation: the policy types Portainer accepts,
// including those added in 3.0, pass validation and unknown ones (or an
// unknown environment type) do not.
func TestPolicyResource_Validation(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	cfg := func(envType, pt string) string {
		return mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_policy" "p" {
  name             = "p"
  environment_type = %q
  policy_type      = %q
}`, envType, pt)
	}
	valid := []string{
		"rbac-k8s", "security-docker", "observability-k8s", "network-security-k8s",
		"vulnerability-scanning-k8s", "automatic-namespace-provisioning-k8s",
	}
	steps := make([]resource.TestStep, 0, len(valid)+2)
	steps = append(steps,
		resource.TestStep{Config: cfg("kubernetes", "vulnerability-scanning-docker"), PlanOnly: true, ExpectError: regexp.MustCompile(`value must be one of`)},
		resource.TestStep{Config: cfg("windows", "rbac-k8s"), PlanOnly: true, ExpectError: regexp.MustCompile(`value must be one of`)},
	)
	for _, pt := range valid {
		steps = append(steps, resource.TestStep{Config: cfg("kubernetes", pt), PlanOnly: true, ExpectNonEmptyPlan: true})
	}
	resource.UnitTest(t, resource.TestCase{ProtoV5ProviderFactories: testProtoV5Factories, Steps: steps})
}

// TestPolicyResource_UpgradeFromV300: state written by v3.0.0 (SDKv2)
// against the same fake API plans no change with this build, with every
// argument set and with the optional ones left out (also when the API
// answers with an empty list for the groups). An API answering {} for the
// data of a policy without data is left out: v3.0.0 itself plans a change
// for it on every run (this build no longer does, see
// TestPolicyResource_MinimalOmitsOptional).
func TestPolicyResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		config     string
		zeroValues bool
	}{
		{"full", fmt.Sprintf(policyFullConfig, "full"), false},
		{"minimal", `
resource "portainer_policy" "p" {
  name             = "minimal"
  environment_type = "docker"
  policy_type      = "security-docker"
}`, false},
		{"minimal_zero_values", `
resource "portainer_policy" "p" {
  name             = "minimal"
  environment_type = "docker"
  policy_type      = "security-docker"
}`, true},
		{"empty_groups_override_false", `
resource "portainer_policy" "p" {
  name               = "empty"
  environment_type   = "swarm"
  policy_type        = "setup-docker"
  environment_groups = []
  allow_override     = false
}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mock := NewMockServer(t)
			fake := newFakePolicies(mock, 1)
			fake.zeroGroups = tc.zeroValues
			config := mockProviderConfig(mock) + tc.config
			resource.Test(t, resource.TestCase{
				IsUnitTest: true,
				Steps: []resource.TestStep{
					{ExternalProviders: previousRelease, Config: config},
					{
						ProtoV5ProviderFactories: testProtoV5Factories,
						Config:                   config,
						ConfigPlanChecks: resource.ConfigPlanChecks{
							PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
						},
					},
				},
			})
		})
	}
}

func TestPolicyAPIFieldsEnvironmentType(t *testing.T) {
	t.Parallel()
	str := func(s string) *string { return &s }
	for name, tc := range map[string]struct {
		f    policyAPIFields
		want *string
	}{
		"singular":  {policyAPIFields{EnvironmentType: str("swarm"), EnvironmentTypes: []string{"docker"}}, str("swarm")},
		"one type":  {policyAPIFields{EnvironmentTypes: []string{"kubernetes"}}, str("kubernetes")},
		"several":   {policyAPIFields{EnvironmentTypes: []string{"docker", "podman", "swarm"}}, nil},
		"no answer": {policyAPIFields{}, nil},
	} {
		got := tc.f.environmentType()
		if (got == nil) != (tc.want == nil) || (got != nil && *got != *tc.want) {
			t.Errorf("%s: got %v, want %v", name, got, tc.want)
		}
	}
}

// TestAccPolicyResource_basic needs Portainer Business Edition (policies are
// a BE feature).
func TestAccPolicyResource_basic(t *testing.T) {
	if os.Getenv("PORTAINER_ACC_BE") != "1" {
		t.Skip("portainer_policy needs Portainer Business Edition; set PORTAINER_ACC_BE=1")
	}
	name := acctest.RandomWithPrefix("acc-policy")
	config := accProvider() + fmt.Sprintf(`
resource "portainer_policy" "p" {
  name             = %q
  environment_type = "docker"
  policy_type      = "cleanup-docker"
}`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config, Check: resource.TestCheckResourceAttr("portainer_policy.p", "name", name)},
			// Portainer returns EnvironmentTypes (["docker","podman","swarm"]
			// for every Docker policy type), not the configured
			// environment_type, so an import cannot tell it for this policy.
			{ResourceName: "portainer_policy.p", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"allow_override", "data", "environment_type"}},
		},
	})
}
