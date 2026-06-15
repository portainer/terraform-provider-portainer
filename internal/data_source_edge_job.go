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

func dataSourceEdgeJob() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceEdgeJobRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the edge job to look up in Portainer.",
			},
			"cron_expression": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Cron expression that schedules execution of the edge job.",
			},
		},
	}
}

func dataSourceEdgeJobRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	name := d.Get("name").(string)

	resp, err := client.DoRequest("GET", "/edge_jobs", nil, nil)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to list edge jobs: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to list edge jobs, status %d: %s", resp.StatusCode, string(data)))
	}

	var jobs []struct {
		ID             int    `json:"Id"`
		Name           string `json:"Name"`
		CronExpression string `json:"CronExpression"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		return diag.FromErr(fmt.Errorf("failed to decode edge job list: %w", err))
	}

	for _, j := range jobs {
		if j.Name == name {
			d.SetId(strconv.Itoa(j.ID))
			if err := d.Set("cron_expression", j.CronExpression); err != nil {
				return diag.FromErr(err)
			}
			return nil
		}
	}

	return diag.FromErr(fmt.Errorf("edge job %s not found", name))
}
