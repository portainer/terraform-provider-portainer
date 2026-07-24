package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceDockerConfig() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceDockerConfigRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Portainer endpoint hosting the Docker config.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the Docker config to look up.",
			},
		},
	}
}

func dataSourceDockerConfigRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	name := d.Get("name").(string)

	path := fmt.Sprintf("/endpoints/%d/docker/configs", endpointID)
	var configs []struct {
		ID   string `json:"ID"`
		Spec struct {
			Name string `json:"Name"`
		} `json:"Spec"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+path, nil, &configs); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list docker configs: %w", err))
	}

	for _, c := range configs {
		if c.Spec.Name == name {
			d.SetId(c.ID)
			return nil
		}
	}

	return diag.FromErr(fmt.Errorf("docker config %s not found in endpoint %d", name, endpointID))
}
