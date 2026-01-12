package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceEdgeGroup() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceEdgeGroupRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"dynamic": {
				Type:     schema.TypeBool,
				Computed: true,
			},
		},
	}
}

func dataSourceEdgeGroupRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	name := d.Get("name").(string)

	resp, err := client.DoRequest("GET", "/edge_groups", nil, nil)
	if err != nil {
		return fmt.Errorf("failed to list edge groups: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to list edge groups, status %d: %s", resp.StatusCode, string(data))
	}

	var groups []struct {
		ID      int    `json:"Id"`
		Name    string `json:"Name"`
		Dynamic bool   `json:"Dynamic"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&groups); err != nil {
		return fmt.Errorf("failed to decode edge group list: %w", err)
	}

	for _, g := range groups {
		if g.Name == name {
			d.SetId(strconv.Itoa(g.ID))
			d.Set("dynamic", g.Dynamic)
			return nil
		}
	}

	return fmt.Errorf("edge group %s not found", name)
}
