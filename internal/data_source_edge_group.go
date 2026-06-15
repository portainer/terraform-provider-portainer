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

func dataSourceEdgeGroup() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceEdgeGroupRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the edge group to look up in Portainer.",
			},
			"dynamic": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the edge group is dynamic (membership computed from tags) or static (explicit endpoint list).",
			},
		},
	}
}

func dataSourceEdgeGroupRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	name := d.Get("name").(string)

	resp, err := client.DoRequest("GET", "/edge_groups", nil, nil)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to list edge groups: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to list edge groups, status %d: %s", resp.StatusCode, string(data)))
	}

	var groups []struct {
		ID      int    `json:"Id"`
		Name    string `json:"Name"`
		Dynamic bool   `json:"Dynamic"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&groups); err != nil {
		return diag.FromErr(fmt.Errorf("failed to decode edge group list: %w", err))
	}

	for _, g := range groups {
		if g.Name == name {
			d.SetId(strconv.Itoa(g.ID))
			if err := d.Set("dynamic", g.Dynamic); err != nil {
				return diag.FromErr(err)
			}
			return nil
		}
	}

	return diag.FromErr(fmt.Errorf("edge group %s not found", name))
}
