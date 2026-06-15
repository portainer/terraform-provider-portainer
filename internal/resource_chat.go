package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return diag.FromErr(err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/chat", client.Endpoint), bytes.NewBuffer(jsonBody))
	if err != nil {
		return diag.FromErr(err)
	}
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return diag.FromErr(fmt.Errorf("no valid authentication method provided (api_key or jwt token)"))
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to send chat: %s", string(body)))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return diag.FromErr(err)
	}

	_ = d.Set("response_message", chatResp.Message)
	_ = d.Set("response_yaml", chatResp.YAML)
	d.SetId(fmt.Sprintf("chat-%d", reqBody.EnvironmentID))
	return nil
}
