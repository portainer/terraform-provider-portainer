package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourcePortainerSupportDebugLog() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePortainerSupportDebugLogApply,
		ReadContext:   resourcePortainerSupportDebugLogRead,
		UpdateContext: resourcePortainerSupportDebugLogApply,
		DeleteContext: resourcePortainerSupportDebugLogDisable,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"enabled": {
				Type:        schema.TypeBool,
				Required:    true,
				Description: "Enable or disable the global debug log",
			},
		},
	}
}

func resourcePortainerSupportDebugLogApply(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	payload := map[string]bool{
		"debugLogEnabled": d.Get("enabled").(bool),
	}

	if err := doJSON(ctx, client, http.MethodPut, fmt.Sprintf("%s/support/debug_log", client.Endpoint), payload, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to set debug log: %w", err))
	}
	d.SetId(strconv.FormatBool(d.Get("enabled").(bool)))
	return nil
}

func resourcePortainerSupportDebugLogRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	var result struct {
		DebugLogEnabled bool `json:"debugLogEnabled"`
	}
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/support/debug_log", client.Endpoint), nil, &result); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read debug log status: %w", err))
	}

	if err := d.Set("enabled", result.DebugLogEnabled); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(strconv.FormatBool(result.DebugLogEnabled))
	return nil
}

func resourcePortainerSupportDebugLogDisable(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	if err := d.Set("enabled", false); err != nil {
		return diag.FromErr(err)
	}
	return resourcePortainerSupportDebugLogApply(ctx, d, meta)
}
