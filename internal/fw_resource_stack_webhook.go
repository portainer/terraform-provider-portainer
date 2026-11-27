package internal

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// portainer_stack_webhook triggers a stack webhook once, at create.

var newStackWebhookResource = dkNewResource(&dkResourceDef{
	typeSuffix: "_stack_webhook",
	schema:     stackWebhookSchema,
	create:     stackWebhookCreate,
})

func stackWebhookSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":         schema.StringAttribute{Computed: true, Description: "Identifier of the resource.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"webhook_id": schema.StringAttribute{Required: true, Description: "UUID of the webhook to trigger", PlanModifiers: []planmodifier.String{dkReplaceString()}},
		},
	}
}

func stackWebhookCreate(ctx context.Context, d *dkBag, client *APIClient) error {
	webhookID := dkString(d, "webhook_id")

	url := fmt.Sprintf("%s/stacks/webhooks/%s", client.Endpoint, webhookID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("failed to build webhook trigger request: %w", err)
	}

	if err := setAuthHeader(req, client); err != nil {
		return err
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to trigger webhook: %w", err)
	}
	defer resp.Body.Close()

	// Portainer 3.0 answers 202 with a JSON body ({stackId, message}), older
	// versions 200 or 204.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to trigger webhook, status %d: %s", resp.StatusCode, string(body))
	}

	d.SetID(webhookID)

	return nil
}
