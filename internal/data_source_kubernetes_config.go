package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceKubernetesConfig() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesConfigRead,

		Schema: map[string]*schema.Schema{
			"environment_ids": {
				Type:        schema.TypeList,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "Environments to include in the generated kubeconfig. Leave unset to include every Kubernetes environment the API token can reach.",
			},
			"exclude_environment_ids": {
				Type:        schema.TypeList,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "Environments to leave out of the generated kubeconfig.",
			},
			// Computed attributes
			"kubeconfig": {
				Type:        schema.TypeString,
				Computed:    true,
				Sensitive:   true,
				Description: "The generated kubeconfig, as YAML. It carries a bearer token for the calling user, so it is a credential in its own right and is stored in state as a sensitive value.",
			},
		},
	}
}

func dataSourceKubernetesConfigRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	q := url.Values{}
	for attr, param := range map[string]string{"environment_ids": "ids", "exclude_environment_ids": "excludeIds"} {
		if v, ok := d.GetOk(attr); ok {
			for _, id := range toIntSlice(v.([]interface{})) {
				q.Add(param, strconv.Itoa(id))
			}
		}
	}
	target := client.Endpoint + "/kubernetes/config"
	if len(q) > 0 {
		target += "?" + q.Encode()
	}

	// The endpoint answers with a YAML document, not JSON, so this goes through
	// the raw byte helper rather than doJSON.
	body, status, err := apiGETWithCodeCtx(ctx, target, client.APIKey, client)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to generate a kubeconfig: %w", err))
	}
	if status >= http.StatusBadRequest {
		return diag.FromErr(fmt.Errorf("failed to generate a kubeconfig: status %d: %s", status, strings.TrimSpace(string(body))))
	}

	if err := d.Set("kubeconfig", string(body)); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("kubernetes-config-" + strconv.FormatInt(makeTimestamp(), 10))
	return nil
}
