package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceCloudCredentials() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceCloudCredentialsRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"cloud_provider": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func dataSourceCloudCredentialsRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	name := d.Get("name").(string)

	resp, err := client.DoRequest("GET", "/cloud/credentials", nil, nil)
	if err != nil {
		return fmt.Errorf("failed to list cloud credentials: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to list cloud credentials, status %d: %s", resp.StatusCode, string(data))
	}

	var credentials []struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Provider string `json:"provider"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&credentials); err != nil {
		return fmt.Errorf("failed to decode cloud credentials list: %w", err)
	}

	for _, c := range credentials {
		if c.Name == name {
			d.SetId(strconv.Itoa(c.ID))
			d.Set("cloud_provider", c.Provider)
			return nil
		}
	}

	return fmt.Errorf("cloud credentials %s not found", name)
}
