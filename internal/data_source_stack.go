package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceStack() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceStackRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the Portainer stack to look up. Combined with endpoint_id to uniquely identify the stack.",
			},
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Portainer environment (endpoint) where the stack is deployed.",
			},
			"type": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Type of the Portainer stack: 1 = Swarm, 2 = Compose, 3 = Kubernetes.",
			},
			"swarm_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Identifier of the Docker Swarm cluster the stack is deployed to. Empty for non-Swarm stacks.",
			},
		},
	}
}

func dataSourceStackRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	name := d.Get("name").(string)
	endpointID := d.Get("endpoint_id").(int)

	resp, err := client.DoRequest("GET", "/stacks", nil, nil)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to list stacks: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to list stacks, status %d: %s", resp.StatusCode, string(data)))
	}

	var stacks []struct {
		ID         int    `json:"Id"`
		Name       string `json:"Name"`
		EndpointID int    `json:"EndpointId"`
		Type       int    `json:"Type"`
		SwarmID    string `json:"SwarmId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&stacks); err != nil {
		return diag.FromErr(fmt.Errorf("failed to decode stack list: %w", err))
	}

	for _, s := range stacks {
		if s.Name == name && s.EndpointID == endpointID {
			d.SetId(strconv.Itoa(s.ID))
			if err := d.Set("type", s.Type); err != nil {
				return diag.FromErr(err)
			}
			if err := d.Set("swarm_id", s.SwarmID); err != nil {
				return diag.FromErr(err)
			}
			return nil
		}
	}

	return diag.FromErr(fmt.Errorf("stack %s not found in endpoint %d", name, endpointID))
}
