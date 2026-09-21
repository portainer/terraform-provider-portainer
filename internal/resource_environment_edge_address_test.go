package internal

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// =========================================================================
// Coverage for the two Edge Agent behaviours that Portainer's own API forces
// on this resource:
//
//   - TLS must not be sent when creating an edge environment. Since 2.43
//     Portainer rejects the request outright, and tls_enabled defaults to true
//     here, so sending it unconditionally broke every edge creation.
//   - A real address change cannot be applied in place, because Portainer bakes
//     the address into the edge key at creation. CustomizeDiff forces
//     replacement so the plan converges instead of repeating forever.
// =========================================================================

// edgeEnvDiff runs the resource's diff for an existing environment, so the
// CustomizeDiff hook is exercised the way Terraform exercises it during a plan.
func edgeEnvDiff(t *testing.T, envType string, stateAddr, configAddr string) *terraform.InstanceDiff {
	t.Helper()
	r := resourceEnvironment()

	state := &terraform.InstanceState{
		ID: "30",
		Attributes: map[string]string{
			"id":                  "30",
			"name":                "edge-prod",
			"type":                envType,
			"group_id":            "1",
			"environment_address": stateAddr,
		},
	}
	config := terraform.NewResourceConfigRaw(map[string]interface{}{
		"name":                "edge-prod",
		"type":                envType,
		"environment_address": configAddr,
	})

	diff, err := r.Diff(context.Background(), state, config, nil)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	return diff
}

// TestEnvironmentDiff_EdgeAgentAddressChangeForcesReplacement is the reason
// CustomizeDiff exists: Portainer cannot move an edge environment in place, so
// a genuine address change has to replace it. Without this the plan showed the
// same change on every run, because Read writes the address back from the edge
// key.
func TestEnvironmentDiff_EdgeAgentAddressChangeForcesReplacement(t *testing.T) {
	diff := edgeEnvDiff(t, "4", "https://portainer.example.com", "https://other.example.com")

	if diff == nil {
		t.Fatal("expected a diff for a changed address")
	}
	if !diff.RequiresNew() {
		t.Error("a real address change on an Edge Agent environment must force replacement")
	}
}

// TestEnvironmentDiff_EdgeAgentSchemeOnlyDoesNotReplace guards the heuristic.
// State written by an older provider version holds Portainer's host-only
// normalisation, so it differs from the configured address by the scheme alone.
// Replacing a live environment — and forcing an agent redeploy — over that would
// be destructive for no reason.
func TestEnvironmentDiff_EdgeAgentSchemeOnlyDoesNotReplace(t *testing.T) {
	diff := edgeEnvDiff(t, "4", "portainer.example.com", "https://portainer.example.com")

	if diff != nil && diff.RequiresNew() {
		t.Error("a scheme-only difference must not replace the environment")
	}
}

// TestEnvironmentDiff_NonEdgeAddressChangeDoesNotReplace verifies the hook is
// edge-specific: a directly-connected environment really does accept a new URL
// on update, so it must keep updating in place.
func TestEnvironmentDiff_NonEdgeAddressChangeDoesNotReplace(t *testing.T) {
	diff := edgeEnvDiff(t, "1", "tcp://docker.example.com:2375", "tcp://other.example.com:2375")

	if diff == nil {
		t.Fatal("expected a diff for a changed address")
	}
	if diff.RequiresNew() {
		t.Error("a non-edge environment must be updated in place, not replaced")
	}
}

// TestEnvironmentDiff_CreationDoesNotForceNew covers the guard for a resource
// that does not exist yet: there is nothing to replace, and reporting a forced
// replacement on creation would be nonsense.
func TestEnvironmentDiff_CreationDoesNotForceNew(t *testing.T) {
	r := resourceEnvironment()
	config := terraform.NewResourceConfigRaw(map[string]interface{}{
		"name":                "edge-prod",
		"type":                "4",
		"environment_address": "https://portainer.example.com",
	})

	diff, err := r.Diff(context.Background(), nil, config, nil)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	if diff != nil && diff.RequiresNew() {
		t.Error("creation must not be reported as a replacement")
	}
}

// TestEnvironmentCreate_EdgeAgentOmitsTLS is the regression test for the
// creation failure: Portainer answers 400 "TLS is not supported for Edge Agent
// environments" as soon as the payload carries TLS, and tls_enabled defaults to
// true, so the fields must not be sent for edge types at all.
func TestEnvironmentCreate_EdgeAgentOmitsTLS(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/endpoints", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 40, "Name": "edge-prod", "Type": 4, "EdgeKey": "k",
	}))
	mock.On("GET", "/endpoints/40", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 40, "Name": "edge-prod", "Type": 4, "GroupId": 1,
		"EdgeKey": "k", "TagIds": []int{},
	}))

	r := resourceEnvironment()
	d := r.TestResourceData()
	_ = d.Set("name", "edge-prod")
	_ = d.Set("environment_address", "https://portainer.example.com")
	_ = d.Set("type", 4)
	// The values a user would most plausibly write, and the schema defaults.
	_ = d.Set("tls_enabled", true)
	_ = d.Set("tls_skip_verify", true)
	_ = d.Set("tls_skip_client_verify", true)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	post := mock.FindRequest("POST", "/endpoints")
	if post == nil {
		t.Fatal("expected POST /endpoints")
	}
	body := string(post.Body)
	for _, field := range []string{"TLS", "TLSSkipVerify", "TLSSkipClientVerify"} {
		if strings.Contains(body, `name="`+field+`"`) {
			t.Errorf("multipart body must not carry %q for an Edge Agent environment", field)
		}
	}
	// The rest of the payload must still be there.
	if !strings.Contains(body, `name="EndpointCreationType"`) {
		t.Error("expected the creation type to still be sent")
	}
}

// TestEnvironmentCreate_NonEdgeStillSendsTLS is the counterpart: dropping TLS
// must stay scoped to edge types, since it is exactly how a directly-connected
// environment is secured.
func TestEnvironmentCreate_NonEdgeStillSendsTLS(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/endpoints", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 41, "Name": "docker", "Type": 1,
	}))
	mock.On("GET", "/endpoints/41", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 41, "Name": "docker", "Type": 1, "GroupId": 1,
		"URL": "tcp://docker.example.com:2375", "TagIds": []int{},
	}))

	r := resourceEnvironment()
	d := r.TestResourceData()
	_ = d.Set("name", "docker")
	_ = d.Set("environment_address", "tcp://docker.example.com:2375")
	_ = d.Set("type", 1)
	_ = d.Set("tls_enabled", true)
	_ = d.Set("tls_skip_verify", true)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	body := string(mock.FindRequest("POST", "/endpoints").Body)
	if !strings.Contains(body, `name="TLS"`) {
		t.Error("a directly-connected environment must still send TLS")
	}
}
