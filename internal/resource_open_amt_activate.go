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
	if err := doJSON(ctx, client, http.MethodPost, url, nil, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to activate OpenAMT: %w", err))
	}

	d.SetId("openamt-" + strconv.Itoa(id))
	return nil
}
