package internal

import (
	"testing"
)

// =========================================================================
// Additional coverage (cov2) for resource_webhook_execute.go: the no-op Read
// handler, which always returns nil and issues no requests.
// =========================================================================

// TestWebhookExecuteCov2_Read_NoOp verifies the Read handler is a pure no-op:
// it returns nil, leaves the ID untouched, and sends no HTTP requests.
func TestWebhookExecuteCov2_Read_NoOp(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceWebhookExecute()
	d := r.TestResourceData()
	d.SetId("tok-xyz")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read should be a no-op, got error: %v", err)
	}
	if d.Id() != "tok-xyz" {
		t.Errorf("expected ID unchanged, got %q", d.Id())
	}
	if len(mock.Requests()) != 0 {
		t.Errorf("expected zero HTTP requests for Read, got %d", len(mock.Requests()))
	}
}
