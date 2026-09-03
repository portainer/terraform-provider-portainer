package internal

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// =========================================================================
// Coverage for the warning resourceEnvironmentUpdate raises when an Edge Agent
// environment's address is edited. The branch is gated on
// d.HasChange("environment_address"), and a plain TestResourceData carries no
// diff, so HasChange is always false there. These tests build a ResourceData
// with a real InstanceState + InstanceDiff via the SDK's InternalMap, the same
// approach as resource_webhook_cov3_test.go.
//
// The warning is also dropped by the rcUpdate adapter (which folds only
// Error-severity diagnostics into an error), so these call UpdateContext
// directly and inspect the diagnostics.
// =========================================================================

// edgeEnvDataWithAddressChange returns a *schema.ResourceData for an Edge Agent
// environment whose environment_address differs between state (old) and diff
// (new), so HasChange("environment_address") reports true.
func edgeEnvDataWithAddressChange(t *testing.T, id, oldAddr, newAddr string) *schema.ResourceData {
	t.Helper()
	r := resourceEnvironment()
	state := &terraform.InstanceState{
		ID: id,
		Attributes: map[string]string{
			"id":                  id,
			"name":                "edge-prod",
			"type":                "4",
			"group_id":            "1",
			"environment_address": oldAddr,
		},
	}
	diff := &terraform.InstanceDiff{
		Attributes: map[string]*terraform.ResourceAttrDiff{
			"environment_address": {Old: oldAddr, New: newAddr},
		},
	}
	d, err := schema.InternalMap(r.Schema).Data(state, diff)
	if err != nil {
		t.Fatalf("failed to build diffed ResourceData: %v", err)
	}
	return d
}

// mockEdgeEnvUpdate registers the PUT + follow-up GET an Update performs, with
// the API reporting the host-only URL Portainer stores for edge environments.
func mockEdgeEnvUpdate(t *testing.T, id string, apiURL, edgeKey string) *MockServer {
	t.Helper()
	mock := NewMockServer(t)
	mock.On("PUT", "/endpoints/"+id, RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 30, "Name": "edge-prod", "Type": 4,
	}))
	mock.On("GET", "/endpoints/"+id, RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 30, "Name": "edge-prod", "Type": 4, "GroupId": 1,
		"URL": apiURL, "EdgeKey": edgeKey, "TagIds": []int{},
	}))
	return mock
}

func warningsIn(diags diag.Diagnostics) []diag.Diagnostic {
	var out []diag.Diagnostic
	for _, d := range diags {
		if d.Severity == diag.Warning {
			out = append(out, d)
		}
	}
	return out
}

// TestEnvironmentUpdate_EdgeAgentAddressChangeWarns verifies that editing an
// Edge Agent address produces a warning rather than a silently successful
// apply: Portainer bakes the address into the edge key at create time and never
// regenerates it, so the running agent keeps using the old address.
func TestEnvironmentUpdate_EdgeAgentAddressChangeWarns(t *testing.T) {
	mock := mockEdgeEnvUpdate(t, "30", "portainer.example.com", "")

	r := resourceEnvironment()
	d := edgeEnvDataWithAddressChange(t, "30", "https://portainer.example.com", "https://other.example.com")

	diags := r.UpdateContext(context.Background(), d, mock.Client())
	if diags.HasError() {
		t.Fatalf("Update failed: %v", diags)
	}

	warnings := warningsIn(diags)
	if len(warnings) != 1 {
		t.Fatalf("expected exactly one warning about the unapplied address, got %d: %v", len(warnings), diags)
	}
	if !strings.Contains(warnings[0].Summary, "environment_address") {
		t.Errorf("warning should name the attribute, got %q", warnings[0].Summary)
	}
	if !strings.Contains(warnings[0].Detail, "-replace") {
		t.Errorf("warning should tell the operator how to fix it, got %q", warnings[0].Detail)
	}
}

// TestEnvironmentUpdate_EdgeAgentSchemeOnlyChangeDoesNotWarn guards the
// heuristic that decides what counts as a real edit. An environment whose state
// still holds Portainer's host-only normalisation (issue #136) differs from the
// configured address by the scheme alone; warning there would fire on every
// apply and train operators to ignore the warning that matters.
func TestEnvironmentUpdate_EdgeAgentSchemeOnlyChangeDoesNotWarn(t *testing.T) {
	mock := mockEdgeEnvUpdate(t, "31", "portainer.example.com", "")

	r := resourceEnvironment()
	d := edgeEnvDataWithAddressChange(t, "31", "portainer.example.com", "https://portainer.example.com")

	diags := r.UpdateContext(context.Background(), d, mock.Client())
	if diags.HasError() {
		t.Fatalf("Update failed: %v", diags)
	}
	if warnings := warningsIn(diags); len(warnings) != 0 {
		t.Errorf("a scheme-only difference must not warn, got: %v", warnings)
	}
}

// TestEnvironmentUpdate_NonEdgeAddressChangeDoesNotWarn verifies the warning is
// edge-specific: for a directly-connected environment the address really is
// sent on update, so there is nothing to warn about.
func TestEnvironmentUpdate_NonEdgeAddressChangeDoesNotWarn(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/endpoints/32", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 32, "Name": "docker", "Type": 1,
	}))
	mock.On("GET", "/endpoints/32", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 32, "Name": "docker", "Type": 1, "GroupId": 1,
		"URL": "tcp://other.example.com:2375", "TagIds": []int{},
	}))

	r := resourceEnvironment()
	state := &terraform.InstanceState{
		ID: "32",
		Attributes: map[string]string{
			"id": "32", "name": "docker", "type": "1", "group_id": "1",
			"environment_address": "tcp://docker.example.com:2375",
		},
	}
	diff := &terraform.InstanceDiff{
		Attributes: map[string]*terraform.ResourceAttrDiff{
			"environment_address": {Old: "tcp://docker.example.com:2375", New: "tcp://other.example.com:2375"},
		},
	}
	d, err := schema.InternalMap(r.Schema).Data(state, diff)
	if err != nil {
		t.Fatalf("failed to build diffed ResourceData: %v", err)
	}

	diags := r.UpdateContext(context.Background(), d, mock.Client())
	if diags.HasError() {
		t.Fatalf("Update failed: %v", diags)
	}
	if warnings := warningsIn(diags); len(warnings) != 0 {
		t.Errorf("a non-edge environment must not warn, got: %v", warnings)
	}
}
