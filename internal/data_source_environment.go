package internal

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/portainer/client-api-go/v2/pkg/client/endpoints"
)

func dataSourceEnvironment() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceEnvironmentRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the Portainer environment to look up. The data source will fail if no matching environment is found.",
			},
			"type": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Type of the Portainer environment: 1 = Docker, 2 = Agent, 3 = Azure, 4 = Edge Agent, 5 = Kubernetes, 6 = Kubernetes via agent, 7 = Kubernetes Edge Agent.",
			},
			"environment_address": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "URL or address used by Portainer to reach the environment.",
			},
			"group_id": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Identifier of the Portainer endpoint group that the environment belongs to.",
			},
		},
	}
}

func dataSourceEnvironmentRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	name := d.Get("name").(string)

	ctx, errBody := withErrorCapture(context.Background())
	params := endpoints.NewEndpointListParams()
	params.SetContext(ctx)
	resp, err := client.Client.Endpoints.EndpointList(params, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to list environments: %w", decorateSDKError(err, errBody))
	}

	for _, e := range resp.Payload {
		if e.Name == name {
			d.SetId(strconv.FormatInt(e.ID, 10))
			d.Set("type", int(e.Type))
			d.Set("environment_address", e.URL)
			d.Set("group_id", int(e.GroupID))
			return nil
		}
	}

	return fmt.Errorf("environment %s not found", name)
}
