package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceEdgeStack() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceEdgeStackRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"deployment_type": {
				Type:     schema.TypeInt,
				Computed: true,
			},
		},
	}
}

func dataSourceEdgeStackRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	name := d.Get("name").(string)

	resp, err := client.DoRequest("GET", "/edge_stacks", nil, nil)
	if err != nil {
		return fmt.Errorf("failed to list edge stacks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to list edge stacks, status %d: %s", resp.StatusCode, string(data))
	}

	var stacks []struct {
		ID             int    `json:"Id"`
		Name           string `json:"Name"`
		DeploymentType int    `json:"DeploymentType"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&stacks); err != nil {
		return fmt.Errorf("failed to decode edge stack list: %w", err)
	}

	for _, s := range stacks {
		if s.Name == name {
			d.SetId(strconv.Itoa(s.ID))
			d.Set("deployment_type", s.DeploymentType)
			return nil
		}
	}

	return fmt.Errorf("edge stack %s not found", name)
}
