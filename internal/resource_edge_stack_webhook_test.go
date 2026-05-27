package internal

import (
	"net/http"
	"testing"
)

// resource_edge_stack_webhook is an action-style resource: Create triggers
// a POST to /edge_stacks/webhooks/<uuid>, Read/Delete are no-ops. The webhook
// UUID is used as the resource ID.

// TestEdgeStackWebhookCreate_HappyPath verifies the POST is sent and ID is set.
func TestEdgeStackWebhookCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	webhookID := "abc-123-uuid"

	mock.On("POST", "/edge_stacks/webhooks/"+webhookID, RespondString(http.StatusNoContent, "", ""))

	r := resourcePortainerEdgeStackWebhook()
	d := r.TestResourceData()
	_ = d.Set("webhook_id", webhookID)

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != webhookID {
		t.Errorf("expected ID %q, got %q", webhookID, d.Id())
	}

	if mock.FindRequest("POST", "/edge_stacks/webhooks/"+webhookID) == nil {
		t.Error("expected POST /edge_stacks/webhooks/<uuid> to be sent")
	}
}

// TestEdgeStackWebhookCreate_200OK verifies that a 200 OK status is also accepted.
func TestEdgeStackWebhookCreate_200OK(t *testing.T) {
	mock := NewMockServer(t)

	webhookID := "ok-uuid"
	mock.On("POST", "/edge_stacks/webhooks/"+webhookID, RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourcePortainerEdgeStackWebhook()
	d := r.TestResourceData()
	_ = d.Set("webhook_id", webhookID)

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != webhookID {
		t.Errorf("expected ID %q, got %q", webhookID, d.Id())
	}
}

// TestEdgeStackWebhookCreate_HTTPError verifies a non-2xx propagates as error.
func TestEdgeStackWebhookCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	webhookID := "missing-uuid"
	mock.On("POST", "/edge_stacks/webhooks/"+webhookID, RespondString(http.StatusNotFound, "application/json", `{"message":"webhook not found"}`))

	r := resourcePortainerEdgeStackWebhook()
	d := r.TestResourceData()
	_ = d.Set("webhook_id", webhookID)

	err := r.Create(d, mock.Client())
	if err == nil {
		t.Fatal("expected error on HTTP 404, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}

// TestEdgeStackWebhookRead_NoOp verifies Read is a no-op (no HTTP traffic).
func TestEdgeStackWebhookRead_NoOp(t *testing.T) {
	mock := NewMockServer(t)

	r := resourcePortainerEdgeStackWebhook()
	d := r.TestResourceData()
	d.SetId("any-id")

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read should be no-op, got error: %v", err)
	}

	if got := len(mock.Requests()); got != 0 {
		t.Errorf("expected zero HTTP calls from Read, got %d", got)
	}
}

// TestEdgeStackWebhookDelete_NoOp verifies Delete is a no-op (no HTTP traffic).
func TestEdgeStackWebhookDelete_NoOp(t *testing.T) {
	mock := NewMockServer(t)

	r := resourcePortainerEdgeStackWebhook()
	d := r.TestResourceData()
	d.SetId("any-id")

	if err := r.Delete(d, mock.Client()); err != nil {
		t.Fatalf("Delete should be no-op, got error: %v", err)
	}

	if got := len(mock.Requests()); got != 0 {
		t.Errorf("expected zero HTTP calls from Delete, got %d", got)
	}
}
