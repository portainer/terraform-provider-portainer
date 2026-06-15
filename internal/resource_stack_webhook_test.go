package internal

import (
	"net/http"
	"testing"
)

// resource_stack_webhook is an action-style resource analogous to
// resource_edge_stack_webhook: Create POSTs to /stacks/webhooks/<uuid>,
// Read/Delete are no-ops.

// TestStackWebhookCreate_HappyPath verifies POST is sent and ID is set.
func TestStackWebhookCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	webhookID := "stack-webhook-uuid-1"
	mock.On("POST", "/stacks/webhooks/"+webhookID, RespondString(http.StatusNoContent, "", ""))

	r := resourcePortainerStackWebhook()
	d := r.TestResourceData()
	_ = d.Set("webhook_id", webhookID)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != webhookID {
		t.Errorf("expected ID %q, got %q", webhookID, d.Id())
	}

	if mock.FindRequest("POST", "/stacks/webhooks/"+webhookID) == nil {
		t.Error("expected POST /stacks/webhooks/<uuid> to be sent")
	}
}

// TestStackWebhookCreate_200OK verifies a 200 OK status is also accepted.
func TestStackWebhookCreate_200OK(t *testing.T) {
	mock := NewMockServer(t)

	webhookID := "ok-uuid"
	mock.On("POST", "/stacks/webhooks/"+webhookID, RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourcePortainerStackWebhook()
	d := r.TestResourceData()
	_ = d.Set("webhook_id", webhookID)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != webhookID {
		t.Errorf("expected ID %q, got %q", webhookID, d.Id())
	}
}

// TestStackWebhookCreate_HTTPError verifies non-2xx propagates as error.
func TestStackWebhookCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	webhookID := "missing-uuid"
	mock.On("POST", "/stacks/webhooks/"+webhookID, RespondString(http.StatusNotFound, "application/json", `{"message":"webhook not found"}`))

	r := resourcePortainerStackWebhook()
	d := r.TestResourceData()
	_ = d.Set("webhook_id", webhookID)

	err := rcCreate(r, d, mock.Client())
	if err == nil {
		t.Fatal("expected error on HTTP 404, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}

// TestStackWebhookRead_NoOp verifies Read is a no-op.
func TestStackWebhookRead_NoOp(t *testing.T) {
	mock := NewMockServer(t)

	r := resourcePortainerStackWebhook()
	d := r.TestResourceData()
	d.SetId("any-id")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read should be no-op, got error: %v", err)
	}
	if got := len(mock.Requests()); got != 0 {
		t.Errorf("expected zero HTTP calls from Read, got %d", got)
	}
}

// TestStackWebhookDelete_NoOp verifies Delete is a no-op.
func TestStackWebhookDelete_NoOp(t *testing.T) {
	mock := NewMockServer(t)

	r := resourcePortainerStackWebhook()
	d := r.TestResourceData()
	d.SetId("any-id")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete should be no-op, got error: %v", err)
	}
	if got := len(mock.Requests()); got != 0 {
		t.Errorf("expected zero HTTP calls from Delete, got %d", got)
	}
}
