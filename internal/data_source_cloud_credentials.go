package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceCloudCredentials() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceCloudCredentialsRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the cloud credentials entry to look up in Portainer.",
			},
			"cloud_provider": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Cloud provider associated with the credentials (e.g. `civo`, `digitalocean`, `linode`, `gke`, `aws`, `azure`).",
			},
		},
	}
}

func dataSourceCloudCredentialsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	name := d.Get("name").(string)

	var credentials []struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Provider string `json:"provider"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/cloud/credentials", nil, &credentials); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list cloud credentials: %w", err))
	}

	for _, c := range credentials {
		if c.Name == name {
			d.SetId(strconv.Itoa(c.ID))
			if err := d.Set("cloud_provider", c.Provider); err != nil {
				return diag.FromErr(err)
			}
			return nil
		}
	}

	return diag.FromErr(fmt.Errorf("cloud credentials %s not found", name))
}
