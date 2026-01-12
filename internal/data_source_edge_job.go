package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceEdgeJob() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceEdgeJobRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"cron_expression": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func dataSourceEdgeJobRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	name := d.Get("name").(string)

	resp, err := client.DoRequest("GET", "/edge_jobs", nil, nil)
	if err != nil {
		return fmt.Errorf("failed to list edge jobs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to list edge jobs, status %d: %s", resp.StatusCode, string(data))
	}

	var jobs []struct {
		ID             int    `json:"Id"`
		Name           string `json:"Name"`
		CronExpression string `json:"CronExpression"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		return fmt.Errorf("failed to decode edge job list: %w", err)
	}

	for _, j := range jobs {
		if j.Name == name {
			d.SetId(strconv.Itoa(j.ID))
			d.Set("cron_expression", j.CronExpression)
			return nil
		}
	}

	return fmt.Errorf("edge job %s not found", name)
}
