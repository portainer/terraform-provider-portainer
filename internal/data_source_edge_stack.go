package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceEdgeStack() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceEdgeStackRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the edge stack to look up in Portainer.",
			},
			"deployment_type": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Deployment type of the edge stack (0 = Compose, 1 = Kubernetes, 2 = Nomad).",
			},
		},
	}
}

func dataSourceEdgeStackRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	name := d.Get("name").(string)

	var stacks []struct {
		ID             int    `json:"Id"`
		Name           string `json:"Name"`
		DeploymentType int    `json:"DeploymentType"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/edge_stacks", nil, &stacks); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list edge stacks: %w", err))
	}

	for _, s := range stacks {
		if s.Name == name {
			d.SetId(strconv.Itoa(s.ID))
			if err := d.Set("deployment_type", s.DeploymentType); err != nil {
				return diag.FromErr(err)
			}
			return nil
		}
	}

	return diag.FromErr(fmt.Errorf("edge stack %s not found", name))
}
