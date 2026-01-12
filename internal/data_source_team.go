package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceTeam() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceTeamRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
		},
	}
}

func dataSourceTeamRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	teamName := d.Get("name").(string)

	resp, err := client.DoRequest("GET", "/teams", nil, nil)
	if err != nil {
		return fmt.Errorf("failed to list teams: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to list teams, status %d: %s", resp.StatusCode, string(data))
	}

	var teams []struct {
		ID   int    `json:"Id"`
		Name string `json:"Name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&teams); err != nil {
		return fmt.Errorf("failed to decode team list: %w", err)
	}

	for _, t := range teams {
		if t.Name == teamName {
			d.SetId(strconv.Itoa(t.ID))
			return nil
		}
	}

	return fmt.Errorf("team %s not found", teamName)
}
