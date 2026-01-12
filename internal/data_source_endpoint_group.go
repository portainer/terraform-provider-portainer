package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceEndpointGroup() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceEndpointGroupRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"description": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func dataSourceEndpointGroupRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	name := d.Get("name").(string)

	resp, err := client.DoRequest("GET", "/endpoint_groups", nil, nil)
	if err != nil {
		return fmt.Errorf("failed to list endpoint groups: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to list endpoint groups, status %d: %s", resp.StatusCode, string(data))
	}

	var groups []struct {
		ID          int    `json:"Id"`
		Name        string `json:"Name"`
		Description string `json:"Description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&groups); err != nil {
		return fmt.Errorf("failed to decode endpoint group list: %w", err)
	}

	for _, g := range groups {
		if g.Name == name {
			d.SetId(strconv.Itoa(g.ID))
			d.Set("description", g.Description)
			return nil
		}
	}

	return fmt.Errorf("endpoint group %s not found", name)
}
