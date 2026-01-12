package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceDockerNetwork() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceDockerNetworkRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:     schema.TypeInt,
				Required: true,
			},
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"driver": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"scope": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func dataSourceDockerNetworkRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	name := d.Get("name").(string)

	path := fmt.Sprintf("/endpoints/%d/docker/networks", endpointID)
	resp, err := client.DoRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to list docker networks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to list docker networks, status %d: %s", resp.StatusCode, string(data))
	}

	var networks []struct {
		ID     string `json:"Id"`
		Name   string `json:"Name"`
		Driver string `json:"Driver"`
		Scope  string `json:"Scope"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&networks); err != nil {
		return fmt.Errorf("failed to decode docker network list: %w", err)
	}

	for _, n := range networks {
		if n.Name == name {
			d.SetId(n.ID)
			d.Set("driver", n.Driver)
			d.Set("scope", n.Scope)
			return nil
		}
	}

	return fmt.Errorf("docker network %s not found in endpoint %d", name, endpointID)
}
