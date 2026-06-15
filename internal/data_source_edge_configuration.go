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

func dataSourceEdgeConfiguration() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceEdgeConfigurationRead,

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

func dataSourceEdgeConfigurationRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	name := d.Get("name").(string)

	resp, err := client.DoRequest("GET", "/edge_configurations", nil, nil)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to list edge configurations: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to list edge configurations, status %d: %s", resp.StatusCode, string(data)))
	}

	var configs []struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Type     int    `json:"type"`
		Category string `json:"category"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&configs); err != nil {
		return diag.FromErr(fmt.Errorf("failed to decode edge configuration list: %w", err))
	}

	for _, c := range configs {
		if c.Name == name {
			d.SetId(strconv.Itoa(c.ID))
			if err := d.Set("type", c.Type); err != nil {
				return diag.FromErr(err)
			}
			if err := d.Set("category", c.Category); err != nil {
				return diag.FromErr(err)
			}
			return nil
		}
	}

	return diag.FromErr(fmt.Errorf("edge configuration %s not found", name))
}
