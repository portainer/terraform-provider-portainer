package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceRegistry() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceRegistryRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"url": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"type": {
				Type:     schema.TypeInt,
				Computed: true,
			},
		},
	}
}

func dataSourceRegistryRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	name := d.Get("name").(string)

	resp, err := client.DoRequest("GET", "/registries", nil, nil)
	if err != nil {
		return fmt.Errorf("failed to list registries: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to list registries, status %d: %s", resp.StatusCode, string(data))
	}

	var registries []struct {
		ID   int    `json:"Id"`
		Name string `json:"Name"`
		URL  string `json:"URL"`
		Type int    `json:"Type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&registries); err != nil {
		return fmt.Errorf("failed to decode registry list: %w", err)
	}

	for _, r := range registries {
		if r.Name == name {
			d.SetId(strconv.Itoa(r.ID))
			d.Set("url", r.URL)
			d.Set("type", r.Type)
			return nil
		}
	}

	return fmt.Errorf("registry %s not found", name)
}
