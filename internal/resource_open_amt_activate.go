package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourcePortainerOpenAMTActivate() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePortainerOpenAMTActivateCreate,
		ReadContext:   schema.NoopContext,
		UpdateContext: schema.NoopContext,
		DeleteContext: removeFromStateContext,
		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "The ID of the environment (endpoint) to activate OpenAMT on.",
			},
		},
	}
}

func resourcePortainerOpenAMTActivateCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id := d.Get("environment_id").(int)

	url := fmt.Sprintf("%s/open_amt/%d/activate", client.Endpoint, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
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
		return diag.FromErr(fmt.Errorf("failed to activate OpenAMT: %s", resp.Status))
	}

	d.SetId("openamt-" + strconv.Itoa(id))
	return nil
}
