package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// The schema is written out rather than derived from gitopsWorkflowAttributes()
// so docsdriftlint can read it statically and keep checking this page for
// schema/doc drift; it is kept in step with portainer_gitops_workflows by
// TestDataSourceGitopsWorkflow_SchemaMatchesList.
func dataSourceGitopsWorkflow() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceGitopsWorkflowRead,

		Schema: map[string]*schema.Schema{
			"workflow_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the GitOps workflow to look up.",
			},
			// Computed attributes
			"name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Name of the workflow.",
			},
			"source_status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Status of the source phase: `healthy`, `syncing`, `error`, `paused` or `unknown`.",
			},
			"source_error": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Error reported by the source phase, empty while healthy.",
			},
			"artifact_status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Status of the artifact phase, using the same values as `source_status`.",
			},
			"artifact_error": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Error reported by the artifact phase, empty while healthy.",
			},
			"target_status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Status of the target (deployment) phase, using the same values as `source_status`.",
			},
			"target_error": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Error reported by the target phase, empty while healthy.",
			},
			"healthy": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether all three phases report `healthy`.",
			},
			"creation_date": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Unix timestamp at which the workflow was created.",
			},
			"last_sync_date": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Unix timestamp of the workflow's last synchronisation.",
			},
		},
	}
}

func dataSourceGitopsWorkflowRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id := d.Get("workflow_id").(int)

	var workflow gitopsWorkflow
	url := fmt.Sprintf("%s/gitops/workflows/%d", client.Endpoint, id)
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &workflow); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read GitOps workflow %d: %w", id, err))
	}

	fields := workflow.toMap()
	// id is carried by the resource ID, not as an attribute of its own.
	delete(fields, "id")
	if err := setFields(d, fields); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(strconv.Itoa(id))
	return nil
}
