package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceCustomTemplate() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceCustomTemplateRead,

		Schema: map[string]*schema.Schema{
			"title": {
				Type:     schema.TypeString,
				Required: true,
			},
			"description": {
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

func dataSourceCustomTemplateRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	title := d.Get("title").(string)

	resp, err := client.DoRequest("GET", "/custom_templates", nil, nil)
	if err != nil {
		return fmt.Errorf("failed to list custom templates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to list custom templates, status %d: %s", resp.StatusCode, string(data))
	}

	var templates []struct {
		ID          int    `json:"Id"`
		Title       string `json:"Title"`
		Description string `json:"Description"`
		Type        int    `json:"Type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&templates); err != nil {
		return fmt.Errorf("failed to decode custom template list: %w", err)
	}

	for _, t := range templates {
		if t.Title == title {
			d.SetId(strconv.Itoa(t.ID))
			d.Set("description", t.Description)
			d.Set("type", t.Type)
			return nil
		}
	}

	return fmt.Errorf("custom template %s not found", title)
}
