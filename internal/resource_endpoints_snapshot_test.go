package internal

import (
	"net/http"
	"testing"
)

// TestEndpointsSnapshotCreate_SingleEndpoint verifies that when endpoint_id
// is set, POST /endpoints/{id}/snapshot is called and the ID is the endpoint
// ID as a string.
func TestEndpointsSnapshotCreate_SingleEndpoint(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/3/snapshot", RespondString(http.StatusNoContent, "", ""))

	r := resourceEndpointsSnapshot()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 3)

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "3" {
		t.Errorf("expected ID %q, got %q", "3", d.Id())
	}
	if mock.FindRequest("POST", "/endpoints/3/snapshot") == nil {
		t.Error("expected POST /endpoints/3/snapshot to be sent")
	}
}

// TestEndpointsSnapshotCreate_AllEndpoints verifies that when endpoint_id is
// omitted, POST /endpoints/snapshot is called and the ID is "all".
func TestEndpointsSnapshotCreate_AllEndpoints(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/snapshot", RespondString(http.StatusNoContent, "", ""))

	r := resourceEndpointsSnapshot()
	d := r.TestResourceData()

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "all" {
		t.Errorf("expected ID %q, got %q", "all", d.Id())
	}
	if mock.FindRequest("POST", "/endpoints/snapshot") == nil {
		t.Error("expected POST /endpoints/snapshot to be sent")
	}
}

// TestEndpointsSnapshotCreate_HTTPError verifies that anything other than
// 204 yields an error.
func TestEndpointsSnapshotCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/9/snapshot", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"boom"}`,
	))

	r := resourceEndpointsSnapshot()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 9)

	if err := r.Create(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestEndpointsSnapshotRead_NoOp verifies that Read is a no-op (action-like
// resource).
func TestEndpointsSnapshotRead_NoOp(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceEndpointsSnapshot()
	d := r.TestResourceData()
	d.SetId("3")

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if len(mock.Requests()) != 0 {
		t.Errorf("expected zero requests for Read, got %d", len(mock.Requests()))
	}
}

// TestEndpointsSnapshotDelete_ClearsID verifies Delete is state-only.
func TestEndpointsSnapshotDelete_ClearsID(t *testing.T) {
	r := resourceEndpointsSnapshot()
	d := r.TestResourceData()
	d.SetId("3")

	if err := r.Delete(d, nil); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}
