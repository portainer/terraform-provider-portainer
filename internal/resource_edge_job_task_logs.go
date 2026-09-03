package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceEdgeJobTaskLogs() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceEdgeJobTaskLogsCreate,
		ReadContext:   schema.NoopContext,
		DeleteContext: resourceEdgeJobTaskLogsDelete,

		Schema: map[string]*schema.Schema{
			"edge_job_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the edge job.",
			},
			"task_id": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the task, from the `portainer_edge_job_tasks` data source.",
			},
		},
	}
}

func resourceEdgeJobTaskLogsCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	jobID := d.Get("edge_job_id").(int)
	taskID := d.Get("task_id").(string)

	// This only asks for collection; the edge agent uploads the logs on its next
	// check-in, so they are not readable the moment this returns.
	url := fmt.Sprintf("%s/edge_jobs/%d/tasks/%s/logs", client.Endpoint, jobID, taskID)
	if err := doJSON(ctx, client, http.MethodPost, url, nil, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to request log collection for task %s of edge job %d: %w", taskID, jobID, err))
	}

	d.SetId(fmt.Sprintf("%d/%s/logs", jobID, taskID))
	return nil
}

func resourceEdgeJobTaskLogsDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	jobID := d.Get("edge_job_id").(int)
	taskID := d.Get("task_id").(string)

	url := fmt.Sprintf("%s/edge_jobs/%d/tasks/%s/logs", client.Endpoint, jobID, taskID)
	if err := doJSON(ctx, client, http.MethodDelete, url, nil, nil); err != nil {
		if !isAPINotFound(err) {
			return diag.FromErr(fmt.Errorf("failed to clear the logs of task %s of edge job %d: %w", taskID, jobID, err))
		}
	}

	d.SetId("")
	return nil
}
