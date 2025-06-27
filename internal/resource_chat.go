package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

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
		Create: resourcePortainerChatSend,
		Read:   schema.Noop,
		Update: schema.Noop,
		Delete: schema.RemoveFromState,
		Schema: map[string]*schema.Schema{
			"context": {
				Type:     schema.TypeString,
				Required: true,
			},
			"environment_id": {
				Type:     schema.TypeInt,
				Required: true,
			},
			"message": {
				Type:     schema.TypeString,
				Required: true,
			},
			"model": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "gpt-3.5-turbo",
			},
			"response_message": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"response_yaml": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourcePortainerChatSend(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	reqBody := ChatRequest{
		Context:       d.Get("context").(string),
		EnvironmentID: d.Get("environment_id").(int),
		Message:       d.Get("message").(string),
		Model:         d.Get("model").(string),
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/chat", client.Endpoint), bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to send chat: %s", string(body))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return err
	}

	d.Set("response_message", chatResp.Message)
	d.Set("response_yaml", chatResp.YAML)
	d.SetId(fmt.Sprintf("chat-%d", reqBody.EnvironmentID))
	return nil
}
