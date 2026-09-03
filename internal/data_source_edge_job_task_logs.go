package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceEdgeJobTaskLogs() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceEdgeJobTaskLogsRead,

		Schema: map[string]*schema.Schema{
			"edge_job_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the edge job.",
			},
			"task_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Identifier of the task, from the `portainer_edge_job_tasks` data source.",
			},
			// Computed attributes
			"logs": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Collected log output of the task. Empty until collection has been requested and the agent has reported back — see `portainer_edge_job_task_logs` (the resource) for triggering it.",
			},
		},
	}
}

func dataSourceEdgeJobTaskLogsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	jobID := d.Get("edge_job_id").(int)
	taskID := d.Get("task_id").(string)

	var body struct {
		FileContent string `json:"FileContent"`
	}
	url := fmt.Sprintf("%s/edge_jobs/%d/tasks/%s/logs", client.Endpoint, jobID, taskID)
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &body); err != nil {
		// Logs that have not been collected yet are absent rather than an error
		// worth failing a plan over.
		if isAPINotFound(err) {
			if err := d.Set("logs", ""); err != nil {
				return diag.FromErr(err)
			}
			d.SetId(fmt.Sprintf("%d/%s/logs", jobID, taskID))
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read the logs of task %s of edge job %d: %w", taskID, jobID, err))
	}

	if err := d.Set("logs", body.FileContent); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(fmt.Sprintf("%d/%s/logs", jobID, taskID))
	return nil
}
