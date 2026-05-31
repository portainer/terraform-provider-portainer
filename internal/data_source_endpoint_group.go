package internal

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/portainer/client-api-go/v2/pkg/client/endpoint_groups"
)

func dataSourceEndpointGroup() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceEndpointGroupRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the Portainer endpoint group to look up. The data source will fail if no matching endpoint group is found.",
			},
			"description": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Description of the Portainer endpoint group as configured in Portainer.",
			},
		},
	}
}

func dataSourceEndpointGroupRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	name := d.Get("name").(string)

	ctx, errBody := withErrorCapture(context.Background())
	params := endpoint_groups.NewEndpointGroupListParams()
	params.SetContext(ctx)
	resp, err := client.Client.EndpointGroups.EndpointGroupList(params, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to list endpoint groups: %w", decorateSDKError(err, errBody))
	}

	for _, g := range resp.Payload {
		if g.Name == name {
			d.SetId(strconv.FormatInt(g.ID, 10))
			d.Set("description", g.Description)
			return nil
		}
	}

	return fmt.Errorf("endpoint group %s not found", name)
}
