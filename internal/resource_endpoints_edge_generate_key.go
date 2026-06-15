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

type GenerateEdgeKeyResponse struct {
	EdgeKey string `json:"edgeKey"`
}

func resourcePortainerEdgeGenerateKey() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePortainerEdgeGenerateKeyCreate,
		ReadContext:   schema.NoopContext,
		DeleteContext: removeFromStateContext,
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

func resourcePortainerEdgeGenerateKeyCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	// Proper JSON payload as required by API
	jsonBody, err := json.Marshal(map[string]string{"edgeKey": ""})
	if err != nil {
		return diag.FromErr(err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/endpoints/edge/generate-key", client.Endpoint), bytes.NewBuffer(jsonBody))
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
		return diag.FromErr(fmt.Errorf("failed to generate edge key: %s", string(body)))
	}

	var result GenerateEdgeKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("edge_key", result.EdgeKey); err != nil {
		return diag.FromErr(err)
	}
	d.SetId("portainer-generated-edge-key")
	return nil
}
