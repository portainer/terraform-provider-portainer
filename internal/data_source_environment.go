package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
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

	resp, err := client.DoRequest("GET", "/endpoints", nil, nil)
	if err != nil {
		return fmt.Errorf("failed to list environments: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to list environments, status %d: %s", resp.StatusCode, string(data))
	}

	var environments []struct {
		ID      int    `json:"Id"`
		Name    string `json:"Name"`
		Type    int    `json:"Type"`
		URL     string `json:"URL"`
		GroupID int    `json:"GroupId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&environments); err != nil {
		return fmt.Errorf("failed to decode environment list: %w", err)
	}

	for _, e := range environments {
		if e.Name == name {
			d.SetId(strconv.Itoa(e.ID))
			d.Set("type", e.Type)
			d.Set("environment_address", e.URL)
			d.Set("group_id", e.GroupID)
			return nil
		}
	}

	return fmt.Errorf("environment %s not found", name)
}
