package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceEdgeConfiguration() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceEdgeConfigurationRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the edge configuration to look up.",
			},
			"type": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Edge configuration type identifier as returned by the Portainer API.",
			},
			"category": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Category of the edge configuration (e.g. `configuration` or `secret`).",
			},
		},
	}
}

func dataSourceEdgeConfigurationRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	name := d.Get("name").(string)

	resp, err := client.DoRequest("GET", "/edge_configurations", nil, nil)
	if err != nil {
		return fmt.Errorf("failed to list edge configurations: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to list edge configurations, status %d: %s", resp.StatusCode, string(data))
	}

	var configs []struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Type     int    `json:"type"`
		Category string `json:"category"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&configs); err != nil {
		return fmt.Errorf("failed to decode edge configuration list: %w", err)
	}

	for _, c := range configs {
		if c.Name == name {
			d.SetId(strconv.Itoa(c.ID))
			d.Set("type", c.Type)
			d.Set("category", c.Category)
			return nil
		}
	}

	return fmt.Errorf("edge configuration %s not found", name)
}
