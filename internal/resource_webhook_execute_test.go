package internal

import (
	"net/http"
	"testing"
)

// TestWebhookExecute_TokenHappyPath verifies that when a "token" is set, the
// resource POSTs to {endpoint}/webhooks/{token} and uses the token as the ID.
func TestWebhookExecute_TokenHappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/webhooks/tok-abc", RespondString(http.StatusNoContent, "", ""))

	r := resourceWebhookExecute()
	d := r.TestResourceData()
	_ = d.Set("token", "tok-abc")

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "tok-abc" {
		t.Errorf("expected ID %q (token), got %q", "tok-abc", d.Id())
	}
	if mock.FindRequest("POST", "/webhooks/tok-abc") == nil {
		t.Error("expected POST /webhooks/tok-abc")
	}
}

// TestWebhookExecute_StackHappyPath verifies the stacks variant.
func TestWebhookExecute_StackHappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/stacks/webhooks/stack-1", RespondString(http.StatusNoContent, "", ""))

	r := resourceWebhookExecute()
	d := r.TestResourceData()
	_ = d.Set("stack_id", "stack-1")

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "stack-1" {
		t.Errorf("expected ID %q, got %q", "stack-1", d.Id())
	}
	if mock.FindRequest("POST", "/stacks/webhooks/stack-1") == nil {
		t.Error("expected POST /stacks/webhooks/stack-1")
	}
}

// TestWebhookExecute_EdgeStackHappyPath verifies the edge_stacks variant.
func TestWebhookExecute_EdgeStackHappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/edge_stacks/webhooks/edge-99", RespondString(http.StatusOK, "", ""))

	r := resourceWebhookExecute()
	d := r.TestResourceData()
	_ = d.Set("edge_stack_id", "edge-99")

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "edge-99" {
		t.Errorf("expected ID %q, got %q", "edge-99", d.Id())
	}
	if mock.FindRequest("POST", "/edge_stacks/webhooks/edge-99") == nil {
		t.Error("expected POST /edge_stacks/webhooks/edge-99")
	}
}

// TestWebhookExecute_NoneSet_Errors verifies the guard that requires exactly
// one of token/stack_id/edge_stack_id to be set.
func TestWebhookExecute_NoneSet_Errors(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceWebhookExecute()
	d := r.TestResourceData()
	// nothing set

	err := r.Create(d, mock.Client())
	if err == nil {
		t.Fatal("expected error when none of token/stack_id/edge_stack_id is set, got nil")
	}
}

// TestWebhookExecute_HTTPError verifies that a 4xx/5xx propagates as an
// error rather than silently succeeding.
func TestWebhookExecute_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/webhooks/bad-token", RespondString(
		http.StatusNotFound, "application/json",
		`{"message":"unknown webhook"}`,
	))

	r := resourceWebhookExecute()
	d := r.TestResourceData()
	_ = d.Set("token", "bad-token")

	if err := r.Create(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 404, got nil")
	}
}

// TestWebhookExecute_Delete_ClearsID verifies the no-op Delete behaviour.
func TestWebhookExecute_Delete_ClearsID(t *testing.T) {
	mock := NewMockServer(t)
	r := resourceWebhookExecute()
	d := r.TestResourceData()
	d.SetId("some-token")

	if err := r.Delete(d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared by Delete, got %q", d.Id())
	}
	if len(mock.Requests()) != 0 {
		t.Errorf("expected zero HTTP requests for Delete, got %d", len(mock.Requests()))
	}
}
