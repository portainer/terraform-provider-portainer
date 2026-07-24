package internal

import (
	"context"
	"fmt"
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
	var result GenerateEdgeKeyResponse
	if err := doJSON(ctx, client, http.MethodPost, fmt.Sprintf("%s/endpoints/edge/generate-key", client.Endpoint), map[string]string{"edgeKey": ""}, &result); err != nil {
		return diag.FromErr(fmt.Errorf("failed to generate edge key: %w", err))
	}

	if err := d.Set("edge_key", result.EdgeKey); err != nil {
		return diag.FromErr(err)
	}
	d.SetId("portainer-generated-edge-key")
	return nil
}
