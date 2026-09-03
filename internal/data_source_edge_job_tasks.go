package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceEdgeJobTasks() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceEdgeJobTasksRead,

		Schema: map[string]*schema.Schema{
			"edge_job_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the edge job whose tasks are listed.",
			},
			// Computed attributes
			"tasks": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "One task per environment the edge job runs on.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id":             {Type: schema.TypeString, Computed: true, Description: "Identifier of the task, used to fetch or clear its logs."},
						"endpoint_id":    {Type: schema.TypeInt, Computed: true, Description: "Environment the task runs on."},
						"endpoint_name":  {Type: schema.TypeString, Computed: true, Description: "Name of that environment."},
						"logs_status":    {Type: schema.TypeInt, Computed: true, Description: "Log collection status: 1 = idle, 2 = pending, 3 = collected."},
						"logs_collected": {Type: schema.TypeBool, Computed: true, Description: "Whether logs have been collected and can be read with `portainer_edge_job_task_logs`."},
					},
				},
			},
		},
	}
}

func dataSourceEdgeJobTasksRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	jobID := d.Get("edge_job_id").(int)

	var list []struct {
		ID           string `json:"Id"`
		EndpointID   int    `json:"EndpointId"`
		EndpointName string `json:"EndpointName"`
		LogsStatus   int    `json:"LogsStatus"`
	}
	url := fmt.Sprintf("%s/edge_jobs/%d/tasks", client.Endpoint, jobID)
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &list); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list the tasks of edge job %d: %w", jobID, err))
	}

	// EdgeJobLogsStatusCollected is 3 in Portainer's enum; anything below means
	// the logs are not there to read yet.
	const logsCollected = 3

	tasks := make([]map[string]interface{}, len(list))
	for i, t := range list {
		tasks[i] = map[string]interface{}{
			"id": t.ID, "endpoint_id": t.EndpointID, "endpoint_name": t.EndpointName,
			"logs_status": t.LogsStatus, "logs_collected": t.LogsStatus == logsCollected,
		}
	}

	if err := d.Set("tasks", tasks); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(strconv.Itoa(jobID) + "/tasks")
	return nil
}
