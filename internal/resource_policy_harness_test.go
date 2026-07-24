package internal

import (
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// These tests demonstrate the table-driven crudCase harness (crud_harness_test.go)
// on the portainer_policy resource. Compare them with the hand-written
// resource_policy_test.go: each case here is a declarative struct instead of the
// ~30 lines of NewMockServer + On(...) + TestResourceData + Set + rcX + assert
// boilerplate the legacy tests repeat.

func TestPolicyHarness_Create(t *testing.T) {
	crudCase{
		Op:       "create",
		Resource: resourcePortainerPolicy(),
		Fields: map[string]interface{}{
			"name":             "p1",
			"environment_type": "kubernetes",
			"policy_type":      "rbac-k8s",
		},
		Routes: []apiRoute{
			{Method: "POST", Path: "/policies", Body: map[string]interface{}{"Id": 12}},
			{Method: "GET", Path: "/policies/12", Body: map[string]interface{}{
				"Name": "p1", "EnvironmentType": "kubernetes", "Type": "rbac-k8s",
			}},
		},
		WantID: "12",
		Check: func(t *testing.T, d *schema.ResourceData) {
			if d.Get("name") != "p1" {
				t.Errorf("name not refreshed from Read: %v", d.Get("name"))
			}
		},
	}.run(t)
}

func TestPolicyHarness_Read404ClearsID(t *testing.T) {
	crudCase{
		Op:       "read",
		Resource: resourcePortainerPolicy(),
		ID:       "99",
		Routes: []apiRoute{
			{Method: "GET", Path: "/policies/99", Status: http.StatusNotFound, Body: `{"message":"gone"}`},
		},
		WantID: "", // 404 → isAPINotFound → d.SetId("")
	}.run(t)
}

func TestPolicyHarness_CreateHTTPError(t *testing.T) {
	crudCase{
		Op:       "create",
		Resource: resourcePortainerPolicy(),
		Fields: map[string]interface{}{
			"name": "p1", "environment_type": "docker", "policy_type": "rbac-docker",
		},
		Routes: []apiRoute{
			{Method: "POST", Path: "/policies", Status: http.StatusBadRequest, Body: `{"message":"bad"}`},
		},
		WantErr:       true,
		WantErrSubstr: "failed to create policy",
	}.run(t)
}

func TestPolicyHarness_Delete(t *testing.T) {
	crudCase{
		Op:       "delete",
		Resource: resourcePortainerPolicy(),
		ID:       "5",
		Routes: []apiRoute{
			{Method: "DELETE", Path: "/policies/5", Status: http.StatusNoContent, Body: ""},
		},
	}.run(t)
}
