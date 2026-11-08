package internal

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceRegistry() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceRegistryRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the Portainer registry to look up. The data source will fail if no matching registry is found.",
			},
			"url": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "URL of the container registry as registered in Portainer.",
			},
			"type": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Type of the Portainer registry (e.g. Quay, Azure, custom, GitLab, ProGet, DockerHub, ECR, GitHub).",
			},
		},
	}
}

func dataSourceRegistryRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	name := d.Get("name").(string)

	registryList, err := listRegistries(ctx, client)
	if err != nil {
		return diag.FromErr(err)
	}

	for _, r := range registryList {
		if r.Name == name {
			d.SetId(strconv.FormatInt(r.ID, 10))
			if err := d.Set("url", r.URL); err != nil {
				return diag.FromErr(err)
			}
			if err := d.Set("type", int(r.Type)); err != nil {
				return diag.FromErr(err)
			}
			return nil
		}
	}

	return diag.FromErr(fmt.Errorf("registry %s not found", name))
}
