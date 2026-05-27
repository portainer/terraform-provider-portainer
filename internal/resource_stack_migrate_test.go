package internal

import (
	"net/http"
	"strings"
	"testing"
)

// resource_stack_migrate is an action-style resource: Create POSTs to
// /stacks/<id>/migrate (via client.DoRequest), Read/Delete are schema.Noop.
// The generated ID has the form "<stackID>-<unix-timestamp>".

// TestStackMigrateCreate_HappyPath verifies the POST payload uses the
// PascalCase JSON keys (EndpointID, Name, SwarmID) expected by Portainer.
func TestStackMigrateCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/stacks/12/migrate", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceStackMigrate()
	d := r.TestResourceData()
	_ = d.Set("stack_id", 12)
	_ = d.Set("target_endpoint_id", 5)
	_ = d.Set("stack_name", "new-name")
	_ = d.Set("swarm_id", "swarm-xyz")

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// ID is "<stack_id>-<unix-timestamp>" — check the prefix.
	if !strings.HasPrefix(d.Id(), "12-") {
		t.Errorf("expected ID starting with %q, got %q", "12-", d.Id())
	}

	post := mock.FindRequest("POST", "/stacks/12/migrate")
	if post == nil {
		t.Fatal("expected POST /stacks/12/migrate")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode POST body: %v", err)
	}
	if got := payload["EndpointID"]; got != float64(5) {
		t.Errorf("payload.EndpointID: expected 5, got %v", got)
	}
	if got := payload["Name"]; got != "new-name" {
		t.Errorf("payload.Name: expected %q, got %v", "new-name", got)
	}
	if got := payload["SwarmID"]; got != "swarm-xyz" {
		t.Errorf("payload.SwarmID: expected %q, got %v", "swarm-xyz", got)
	}
}

// TestStackMigrateCreate_WithSourceEndpoint verifies the source endpoint_id
// is appended as a query parameter.
func TestStackMigrateCreate_WithSourceEndpoint(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/stacks/3/migrate", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceStackMigrate()
	d := r.TestResourceData()
	_ = d.Set("stack_id", 3)
	_ = d.Set("target_endpoint_id", 2)
	_ = d.Set("endpoint_id", 1)

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	req := mock.FindRequest("POST", "/stacks/3/migrate")
	if req == nil {
		t.Fatal("expected POST /stacks/3/migrate")
	}
	if !strings.Contains(req.Query, "endpointId=1") {
		t.Errorf("expected endpointId=1 in query string, got %q", req.Query)
	}
}

// TestStackMigrateCreate_MinimalPayload verifies that optional fields are
// omitted from the payload when not set.
func TestStackMigrateCreate_MinimalPayload(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/stacks/4/migrate", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceStackMigrate()
	d := r.TestResourceData()
	_ = d.Set("stack_id", 4)
	_ = d.Set("target_endpoint_id", 9)

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	post := mock.FindRequest("POST", "/stacks/4/migrate")
	if post == nil {
		t.Fatal("expected POST /stacks/4/migrate")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if _, ok := payload["Name"]; ok {
		t.Errorf("expected Name to be absent when stack_name unset, got %v", payload["Name"])
	}
	if _, ok := payload["SwarmID"]; ok {
		t.Errorf("expected SwarmID to be absent when swarm_id unset, got %v", payload["SwarmID"])
	}
	if got := payload["EndpointID"]; got != float64(9) {
		t.Errorf("payload.EndpointID: expected 9, got %v", got)
	}
}

// TestStackMigrateCreate_HTTPError verifies non-2xx surfaces as an error.
func TestStackMigrateCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/stacks/5/migrate", RespondString(http.StatusBadRequest, "application/json", `{"message":"cannot migrate"}`))

	r := resourceStackMigrate()
	d := r.TestResourceData()
	_ = d.Set("stack_id", 5)
	_ = d.Set("target_endpoint_id", 6)

	err := r.Create(d, mock.Client())
	if err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}
