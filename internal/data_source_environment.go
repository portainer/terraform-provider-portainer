package internal

import (
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
				Type:     schema.TypeString,
				Required: true,
			},
			"type": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"environment_address": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"group_id": {
				Type:     schema.TypeInt,
				Computed: true,
			},
		},
	}
}

func dataSourceEnvironmentRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	name := d.Get("name").(string)

	params := endpoints.NewEndpointListParams()
	resp, err := client.Client.Endpoints.EndpointList(params, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to list environments: %w", err)
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
