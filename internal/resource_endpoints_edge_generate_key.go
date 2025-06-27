package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"io"
	"net/http"
)

type GenerateEdgeKeyResponse struct {
	EdgeKey string `json:"edgeKey"`
}

func resourcePortainerEdgeGenerateKey() *schema.Resource {
	return &schema.Resource{
		Create: resourcePortainerEdgeGenerateKeyCreate,
		Read:   schema.Noop,
		Delete: schema.RemoveFromState,
		Schema: map[string]*schema.Schema{
			"edge_key": {
				Type:        schema.TypeString,
				Computed:    true,
				ForceNew:    true,
				Description: "The generated general edge key.",
			},
		},
	}
}

func resourcePortainerEdgeGenerateKeyCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	// Proper JSON payload as required by API
	jsonBody, err := json.Marshal(map[string]string{"edgeKey": ""})
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/endpoints/edge/generate-key", client.Endpoint), bytes.NewBuffer(jsonBody))
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
		return fmt.Errorf("failed to generate edge key: %s", string(body))
	}

	var result GenerateEdgeKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	d.Set("edge_key", result.EdgeKey)
	d.SetId("portainer-generated-edge-key")
	return nil
}
