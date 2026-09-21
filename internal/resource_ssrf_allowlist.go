package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// ssrfAllowListKey is the only allow list Portainer defines
// (portainer.AllowListKey has the single value AllowListSSRF). The resource
// therefore has no argument for it: an attribute with exactly one legal value
// is noise in every configuration that has to write it out.
const ssrfAllowListKey = 0

// ssrfModes maps the readable mode names this resource takes onto the integers
// Portainer stores (portainer.SSRFMode).
var ssrfModes = map[string]int{
	"off":     0,
	"audit":   1,
	"enforce": 2,
}

// ssrfModeNames is the inverse, for reading state back.
var ssrfModeNames = map[int]string{
	0: "off",
	1: "audit",
	2: "enforce",
}

// resourceSSRFAllowList manages the list of URLs Portainer will make outbound
// proxy requests to, and how strictly the list is applied.
func resourceSSRFAllowList() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceSSRFAllowListWrite,
		ReadContext:   resourceSSRFAllowListRead,
		UpdateContext: resourceSSRFAllowListWrite,
		// Portainer has no endpoint to remove the allow list, and turning a
		// security control off is a decision to make explicitly rather than a
		// side effect of no longer managing it in Terraform. Destroying the
		// resource stops managing the list and leaves it as configured.
		DeleteContext: removeFromStateContext,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"mode": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringInSlice([]string{"off", "audit", "enforce"}, false),
				Description:  "How the allow list is applied: `off` does not check outbound requests at all, `audit` records requests that are not on the list but still makes them, and `enforce` blocks them.",
			},
			"entries": {
				Type:        schema.TypeList,
				Required:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "URLs Portainer is allowed to make outbound proxy requests to. The list is authoritative: a URL missing from it is removed.",
			},
		},
	}
}

func ssrfAllowListURL(client *APIClient) string {
	return fmt.Sprintf("%s/allowlist/%d", client.Endpoint, ssrfAllowListKey)
}

func resourceSSRFAllowListWrite(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	entries, _ := d.Get("entries").([]interface{})
	list := make([]string, 0, len(entries))
	for _, entry := range entries {
		if value, ok := entry.(string); ok {
			list = append(list, value)
		}
	}

	// The endpoint replaces the list outright, so an empty one is a meaningful
	// request rather than something to omit: it allows nothing. Paired with
	// mode `enforce` that blocks every outbound proxy request, which is why the
	// mode is spelled out rather than defaulted.
	payload := map[string]interface{}{
		"Entries": list,
		"Mode":    ssrfModes[d.Get("mode").(string)],
	}

	if err := doJSON(ctx, client, http.MethodPut, ssrfAllowListURL(client), payload, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update the SSRF allow list: %w", err))
	}

	d.SetId("portainer-ssrf-allowlist")
	return resourceSSRFAllowListRead(ctx, d, meta)
}

func resourceSSRFAllowListRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	var allowList struct {
		Entries []string `json:"Entries"`
		Mode    int      `json:"Mode"`
	}
	if err := doJSON(ctx, client, http.MethodGet, ssrfAllowListURL(client), nil, &allowList); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the SSRF allow list: %w", err))
	}

	mode, ok := ssrfModeNames[allowList.Mode]
	if !ok {
		// A mode this provider does not know about is worth saying out loud:
		// silently mapping it onto "off" would misreport a security control.
		return diag.FromErr(fmt.Errorf("unrecognised SSRF mode %d reported by Portainer", allowList.Mode))
	}

	d.SetId("portainer-ssrf-allowlist")
	if err := setFields(d, map[string]interface{}{
		"mode":    mode,
		"entries": allowList.Entries,
	}); err != nil {
		return diag.FromErr(err)
	}
	return nil
}
