package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceStack() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceStackRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"endpoint_id": {
				Type:     schema.TypeInt,
				Required: true,
			},
			"type": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"swarm_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func dataSourceStackRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	name := d.Get("name").(string)
	endpointID := d.Get("endpoint_id").(int)

	resp, err := client.DoRequest("GET", "/stacks", nil, nil)
	if err != nil {
		return fmt.Errorf("failed to list stacks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to list stacks, status %d: %s", resp.StatusCode, string(data))
	}

	var stacks []struct {
		ID         int    `json:"Id"`
		Name       string `json:"Name"`
		EndpointID int    `json:"EndpointId"`
		Type       int    `json:"Type"`
		SwarmID    string `json:"SwarmId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&stacks); err != nil {
		return fmt.Errorf("failed to decode stack list: %w", err)
	}

	for _, s := range stacks {
		if s.Name == name && s.EndpointID == endpointID {
			d.SetId(strconv.Itoa(s.ID))
			d.Set("type", s.Type)
			d.Set("swarm_id", s.SwarmID)
			return nil
		}
	}

	return fmt.Errorf("stack %s not found in endpoint %d", name, endpointID)
}
