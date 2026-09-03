package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// gitopsWorkflow is the workflows.Workflow payload. The three phase statuses
// are what the Portainer UI shows as the workflow's health: a workflow is only
// fully healthy when the source, the artifact and the target all are.
type gitopsWorkflow struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Status struct {
		Source   gitopsPhaseStatus `json:"source"`
		Artifact gitopsPhaseStatus `json:"artifact"`
		Target   gitopsPhaseStatus `json:"target"`
	} `json:"status"`
	CreationDate int `json:"creationDate"`
	LastSyncDate int `json:"lastSyncDate"`
}

type gitopsPhaseStatus struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

func gitopsWorkflowAttributes() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"id":              {Type: schema.TypeInt, Computed: true, Description: "Identifier of the workflow."},
		"name":            {Type: schema.TypeString, Computed: true, Description: "Name of the workflow."},
		"source_status":   {Type: schema.TypeString, Computed: true, Description: "Status of the source phase: `healthy`, `syncing`, `error`, `paused` or `unknown`."},
		"source_error":    {Type: schema.TypeString, Computed: true, Description: "Error reported by the source phase, empty while healthy."},
		"artifact_status": {Type: schema.TypeString, Computed: true, Description: "Status of the artifact phase, using the same values as `source_status`."},
		"artifact_error":  {Type: schema.TypeString, Computed: true, Description: "Error reported by the artifact phase, empty while healthy."},
		"target_status":   {Type: schema.TypeString, Computed: true, Description: "Status of the target (deployment) phase, using the same values as `source_status`."},
		"target_error":    {Type: schema.TypeString, Computed: true, Description: "Error reported by the target phase, empty while healthy."},
		"healthy":         {Type: schema.TypeBool, Computed: true, Description: "Whether all three phases report `healthy`."},
		"creation_date":   {Type: schema.TypeInt, Computed: true, Description: "Unix timestamp at which the workflow was created."},
		"last_sync_date":  {Type: schema.TypeInt, Computed: true, Description: "Unix timestamp of the workflow's last synchronisation."},
	}
}

func (w gitopsWorkflow) toMap() map[string]interface{} {
	return map[string]interface{}{
		"id": w.ID, "name": w.Name,
		"source_status": w.Status.Source.Status, "source_error": w.Status.Source.Error,
		"artifact_status": w.Status.Artifact.Status, "artifact_error": w.Status.Artifact.Error,
		"target_status": w.Status.Target.Status, "target_error": w.Status.Target.Error,
		"healthy": w.Status.Source.Status == "healthy" &&
			w.Status.Artifact.Status == "healthy" &&
			w.Status.Target.Status == "healthy",
		"creation_date": w.CreationDate, "last_sync_date": w.LastSyncDate,
	}
}

func dataSourceGitopsWorkflows() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceGitopsWorkflowsRead,

		Schema: map[string]*schema.Schema{
			"search": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Free-text filter matched against the workflow name.",
			},
			"sort": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Field to sort the results by, as accepted by the Portainer API (for example `name`).",
			},
			"order": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringInSlice([]string{"asc", "desc"}, false),
				Description:  "Sort direction, `asc` or `desc`.",
			},
			"start": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Zero-based index of the first result to return, for paging through a large list.",
			},
			"limit": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Maximum number of results to return. Leave unset to let Portainer decide.",
			},
			"endpoint_ids": {
				Type:        schema.TypeList,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "Only return workflows deploying to these environment identifiers.",
			},
			"status": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringInSlice([]string{"healthy", "syncing", "error", "paused", "unknown"}, false),
				Description:  "Only return workflows in this status.",
			},
			"type": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringInSlice([]string{"stack", "edgeStack"}, false),
				Description:  "Only return workflows of this type.",
			},
			"platform": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringInSlice([]string{"dockerStandalone", "dockerSwarm", "kubernetes"}, false),
				Description:  "Only return workflows deploying to this platform.",
			},
			// Computed attributes
			"workflows": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "GitOps workflows matching the query.",
				Elem:        &schema.Resource{Schema: gitopsWorkflowAttributes()},
			},
			"summary": gitopsStatusSummarySchema("GitOps workflows"),
		},
	}
}

func dataSourceGitopsWorkflowsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	q := url.Values{}
	for attr, param := range map[string]string{
		"search": "search", "sort": "sort", "order": "order",
		"status": "status", "type": "type", "platform": "platform",
	} {
		if v, ok := d.GetOk(attr); ok {
			q.Set(param, v.(string))
		}
	}
	for attr, param := range map[string]string{"start": "start", "limit": "limit"} {
		if v, ok := d.GetOk(attr); ok {
			q.Set(param, strconv.Itoa(v.(int)))
		}
	}
	if v, ok := d.GetOk("endpoint_ids"); ok {
		ids := toIntSlice(v.([]interface{}))
		parts := make([]string, len(ids))
		for i, id := range ids {
			parts[i] = strconv.Itoa(id)
		}
		q.Set("endpointIds", strings.Join(parts, ","))
	}
	query := ""
	if len(q) > 0 {
		query = "?" + q.Encode()
	}

	var list []gitopsWorkflow
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/gitops/workflows"+query, nil, &list); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list GitOps workflows: %w", err))
	}

	workflows := make([]map[string]interface{}, len(list))
	for i, w := range list {
		workflows[i] = w.toMap()
	}

	// The summary endpoint is unfiltered and always describes every workflow.
	var summary gitopsStatusSummary
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/gitops/workflows/summary", nil, &summary); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the GitOps workflow summary: %w", err))
	}

	if err := setFields(d, map[string]interface{}{
		"workflows": workflows,
		"summary":   summary.toList(),
	}); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("gitops-workflows-" + strconv.FormatInt(makeTimestamp(), 10))
	return nil
}
