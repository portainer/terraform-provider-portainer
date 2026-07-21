package internal

import (
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// =========================================================================
// cov3 coverage for resource_webhook.go: the resourceWebhookUpdate PUT branch,
// which is gated on d.HasChange("registry_id"). A plain TestResourceData carries
// no diff, so HasChange is always false there (see TestWebhookUpdate_NoChangeIsNoOp).
// To exercise the PUT branch we build a ResourceData with a real InstanceState +
// InstanceDiff via the SDK's InternalMap so HasChange("registry_id") is true.
// =========================================================================

// webhookDataWithRegistryChange returns a *schema.ResourceData for the webhook
// resource whose registry_id differs between state (old) and diff (new), so
// HasChange("registry_id") reports true and Update performs the PUT.
func webhookDataWithRegistryChange(t *testing.T, id, oldRegistry, newRegistry string) *schema.ResourceData {
	t.Helper()
	r := resourceWebhook()
	state := &terraform.InstanceState{
		ID: id,
		Attributes: map[string]string{
			"id":           id,
			"endpoint_id":  "1",
			"resource_id":  "abc",
			"webhook_type": "1",
			"registry_id":  oldRegistry,
		},
	}
	diff := &terraform.InstanceDiff{
		Attributes: map[string]*terraform.ResourceAttrDiff{
			"registry_id": {Old: oldRegistry, New: newRegistry},
		},
	}
	d, err := schema.InternalMap(r.Schema).Data(state, diff)
	if err != nil {
		t.Fatalf("failed to build diffed ResourceData: %v", err)
	}
	return d
}

// TestWebhookCov3_Update_RegistryChange_HappyPath exercises the PUT branch of
// resourceWebhookUpdate: a changed registry_id triggers PUT /webhooks/{id}.
func TestWebhookCov3_Update_RegistryChange_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/webhooks/9", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 9, "RegistryId": 5,
	}))

	r := resourceWebhook()
	d := webhookDataWithRegistryChange(t, "9", "3", "5")

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if mock.FindRequest("PUT", "/webhooks/9") == nil {
		t.Error("expected PUT /webhooks/9 when registry_id changed")
	}
}

// TestWebhookCov3_Update_RegistryChange_HTTPError covers the error branch where
// the PUT returns a non-2xx status.
func TestWebhookCov3_Update_RegistryChange_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/webhooks/9", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"update boom"}`,
	))

	r := resourceWebhook()
	d := webhookDataWithRegistryChange(t, "9", "3", "5")

	if err := rcUpdate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when PUT /webhooks/9 returns 500, got nil")
	}
	if mock.FindRequest("PUT", "/webhooks/9") == nil {
		t.Error("expected the PUT /webhooks/9 attempt to have been made")
	}
}
