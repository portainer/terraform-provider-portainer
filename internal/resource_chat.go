package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type ChatRequest struct {
	Context       string `json:"context"`
	EnvironmentID int    `json:"environmentID"`
	Message       string `json:"message"`
	Model         string `json:"model"`
}

type ChatResponse struct {
	Message string `json:"message"`
	YAML    string `json:"yaml"`
}

func resourcePortainerChat() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePortainerChatSend,
		ReadContext:   schema.NoopContext,
		UpdateContext: schema.NoopContext,
		DeleteContext: removeFromStateContext,
		Schema: map[string]*schema.Schema{
			"context": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Conversation context identifier passed to the Portainer chat assistant.",
			},
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Portainer environment the chat request is scoped to.",
			},
			"message": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "User prompt sent to the Portainer chat assistant.",
			},
			"model": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "gpt-3.5-turbo",
				Description: "AI model name used to generate the chat response (defaults to `gpt-3.5-turbo`).",
			},
			"response_message": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Textual response returned by the Portainer chat assistant.",
			},
			"response_yaml": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "YAML manifest extracted from the chat response, when applicable.",
			},
		},
	}
}

func resourcePortainerChatSend(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	reqBody := ChatRequest{
		Context:       d.Get("context").(string),
		EnvironmentID: d.Get("environment_id").(int),
		Message:       d.Get("message").(string),
		Model:         d.Get("model").(string),
	}

	var chatResp ChatResponse
	if err := doJSON(ctx, client, http.MethodPost, fmt.Sprintf("%s/chat", client.Endpoint), reqBody, &chatResp); err != nil {
		return diag.FromErr(fmt.Errorf("failed to send chat: %w", err))
	}

	if err := setFields(d, map[string]interface{}{
		"response_message": chatResp.Message,
		"response_yaml":    chatResp.YAML,
	}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("chat-%d", reqBody.EnvironmentID))
	return nil
}
