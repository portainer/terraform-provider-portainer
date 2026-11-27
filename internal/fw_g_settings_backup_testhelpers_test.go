package internal

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// Test helpers of the batch G framework resources (settings, backup, LDAP,
// miscellaneous).

// gsbBody decodes the JSON body of the last method+path request the mock got.
func gsbBody(t *testing.T, mock *MockServer, method, path string) map[string]interface{} {
	t.Helper()
	var last *RecordedRequest
	for _, r := range mock.Requests() {
		if r.Method == method && r.Path == path {
			last = r
		}
	}
	if last == nil {
		t.Fatalf("expected a %s %s request", method, path)
	}
	var body map[string]interface{}
	if err := last.DecodeJSON(&body); err != nil {
		t.Fatalf("decode %s %s body: %v", method, path, err)
	}
	return body
}

// gsbCount counts the method+path requests the mock got.
func gsbCount(mock *MockServer, method, path string) int {
	n := 0
	for _, r := range mock.Requests() {
		if r.Method == method && r.Path == path {
			n++
		}
	}
	return n
}

// gsbUpgradeTest applies each configuration with v3.0.0 against a fresh mock
// from newMock, then plans it with this build, which has to plan no change.
func gsbUpgradeTest(t *testing.T, newMock func(t *testing.T) *MockServer, configs ...string) {
	t.Helper()
	for i, c := range configs {
		t.Run(fmt.Sprintf("config%d", i), func(t *testing.T) {
			mock := newMock(t)
			config := mockProviderConfig(mock) + c
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

func regexpMust(s string) *regexp.Regexp { return regexp.MustCompile(s) }
