package internal

import (
	"net/http"
	"testing"
)

// TestEdgeStackUpdate_KeepsWebhook is the regression test for the webhook being
// replaced on every Git update, which silently broke every caller holding the
// old webhook URL.
func TestEdgeStackUpdate_KeepsWebhook(t *testing.T) {
	const webhook = "11111111-2222-3333-4444-555555555555"
	mock := NewMockServer(t)
	mock.On("PUT", "/edge_stacks/60/git", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 60}))
	mock.On("GET", "/edge_stacks/60", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":         60,
		"Name":       "git-upd",
		"AutoUpdate": map[string]interface{}{"Webhook": webhook},
	}))

	r := resourceEdgeStack()
	d := r.TestResourceData()
	d.SetId("60")
	_ = d.Set("name", "git-upd")
	_ = d.Set("edge_groups", []interface{}{9})
	_ = d.Set("repository_url", "https://github.com/example/repo.git")
	_ = d.Set("repository_reference_name", "refs/heads/release")
	_ = d.Set("stack_webhook", true)
	_ = d.Set("webhook_id", webhook)

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	var payload struct {
		AutoUpdate struct {
			Webhook string `json:"webhook"`
		} `json:"autoUpdate"`
	}
	if err := mock.FindRequest("PUT", "/edge_stacks/60/git").DecodeJSON(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.AutoUpdate.Webhook != webhook {
		t.Errorf("expected the existing webhook %q to be sent, got %q", webhook, payload.AutoUpdate.Webhook)
	}
	if got := d.Get("webhook_id").(string); got != webhook {
		t.Errorf("webhook_id: expected %q, got %q", webhook, got)
	}
}
