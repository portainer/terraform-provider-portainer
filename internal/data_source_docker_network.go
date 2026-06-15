package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceDockerNetwork() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceDockerNetworkRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "ID of the Portainer environment (Docker host or Swarm) where the network is located.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the Docker network to look up.",
			},
			"driver": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Driver used by the Docker network (e.g., bridge, overlay, macvlan, host).",
			},
			"scope": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Scope of the Docker network (e.g., local, global, swarm).",
			},
		},
	}
}

func dataSourceDockerNetworkRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	name := d.Get("name").(string)

	path := fmt.Sprintf("/endpoints/%d/docker/networks", endpointID)
	resp, err := client.DoRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to list docker networks: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to list docker networks, status %d: %s", resp.StatusCode, string(data)))
	}

	var networks []struct {
		ID     string `json:"Id"`
		Name   string `json:"Name"`
		Driver string `json:"Driver"`
		Scope  string `json:"Scope"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&networks); err != nil {
		return diag.FromErr(fmt.Errorf("failed to decode docker network list: %w", err))
	}

	for _, n := range networks {
		if n.Name == name {
			d.SetId(n.ID)
			if err := d.Set("driver", n.Driver); err != nil {
				return diag.FromErr(err)
			}
			if err := d.Set("scope", n.Scope); err != nil {
				return diag.FromErr(err)
			}
			return nil
		}
	}

	return diag.FromErr(fmt.Errorf("docker network %s not found in endpoint %d", name, endpointID))
}
