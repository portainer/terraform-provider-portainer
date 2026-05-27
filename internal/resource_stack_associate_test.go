package internal

import (
	"net/http"
	"strings"
	"testing"
)

// resource_stack_associate is an action-style resource: Create PUTs to
// /stacks/<id>/associate?endpointId=..&swarmId=..&orphanedRunning=..,
// Read is a no-op, Delete just clears the ID locally (no HTTP call).

// TestStackAssociateCreate_HappyPath verifies the PUT is sent with the correct
// query parameters and the ID is set from stack_id.
func TestStackAssociateCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/stacks/7/associate", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceStackAssociate()
	d := r.TestResourceData()
	_ = d.Set("stack_id", 7)
	_ = d.Set("endpoint_id", 3)
	_ = d.Set("swarm_id", "swarm-abc")
	_ = d.Set("orphaned_running", true)

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "7" {
		t.Errorf("expected ID %q (from stack_id), got %q", "7", d.Id())
	}

	req := mock.FindRequest("PUT", "/stacks/7/associate")
	if req == nil {
		t.Fatal("expected PUT /stacks/7/associate")
	}

	// Query params: endpointId=3, swarmId=swarm-abc, orphanedRunning=true.
	if !strings.Contains(req.Query, "endpointId=3") {
		t.Errorf("expected endpointId=3 in query, got %q", req.Query)
	}
	if !strings.Contains(req.Query, "swarmId=swarm-abc") {
		t.Errorf("expected swarmId=swarm-abc in query, got %q", req.Query)
	}
	if !strings.Contains(req.Query, "orphanedRunning=true") {
		t.Errorf("expected orphanedRunning=true in query, got %q", req.Query)
	}
}

// TestStackAssociateCreate_OrphanedRunningDefault verifies the default value
// for orphaned_running (false) ends up in the query string.
func TestStackAssociateCreate_OrphanedRunningDefault(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/stacks/1/associate", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceStackAssociate()
	d := r.TestResourceData()
	_ = d.Set("stack_id", 1)
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("swarm_id", "swarm-x")

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	req := mock.FindRequest("PUT", "/stacks/1/associate")
	if req == nil {
		t.Fatal("expected PUT /stacks/1/associate")
	}
	if !strings.Contains(req.Query, "orphanedRunning=false") {
		t.Errorf("expected orphanedRunning=false in query, got %q", req.Query)
	}
}

// TestStackAssociateCreate_HTTPError verifies non-200 propagates as an error.
func TestStackAssociateCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/stacks/9/associate", RespondString(http.StatusBadRequest, "application/json", `{"message":"invalid"}`))

	r := resourceStackAssociate()
	d := r.TestResourceData()
	_ = d.Set("stack_id", 9)
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("swarm_id", "swarm-x")

	err := r.Create(d, mock.Client())
	if err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}

// TestStackAssociateRead_NoOp verifies Read is a no-op.
func TestStackAssociateRead_NoOp(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceStackAssociate()
	d := r.TestResourceData()
	d.SetId("5")

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read should be no-op, got error: %v", err)
	}
	if got := len(mock.Requests()); got != 0 {
		t.Errorf("expected zero HTTP calls from Read, got %d", got)
	}
}

// TestStackAssociateDelete_ClearsIDOnly verifies Delete clears the ID and
// makes no HTTP calls.
func TestStackAssociateDelete_ClearsIDOnly(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceStackAssociate()
	d := r.TestResourceData()
	d.SetId("5")

	if err := r.Delete(d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared after Delete, got %q", d.Id())
	}
	if got := len(mock.Requests()); got != 0 {
		t.Errorf("expected zero HTTP calls from Delete, got %d", got)
	}
}
