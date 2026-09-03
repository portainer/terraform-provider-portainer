package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceGitopsSource() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceGitopsSourceRead,

		Schema: map[string]*schema.Schema{
			"source_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the GitOps source to look up.",
			},
			// Computed attributes
			"name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Name of the source.",
			},
			"url": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Repository URL the source syncs from.",
			},
			"type": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Source type: `git`, `helm` or `oci`.",
			},
			"status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Sync status: `healthy`, `syncing`, `error`, `paused` or `unknown`.",
			},
			"status_error": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Error reported by the last synchronisation attempt, empty while the source is healthy.",
			},
			"interval": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Polling interval of the source, as a duration string.",
			},
			"last_sync": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Unix timestamp of the last successful synchronisation.",
			},
			"username": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Username the source authenticates with, empty for an anonymous repository. Portainer never returns the password.",
			},
			"tls_skip_verify": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether TLS certificate verification is skipped when contacting the repository.",
			},
			"public": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether every user can use this source.",
			},
			"user_accesses": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "IDs of users granted access to the source.",
			},
			"team_accesses": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "IDs of teams granted access to the source.",
			},
			"workflows": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Workflows deployed from this source. Deleting a source that is still used by a workflow is refused by Portainer.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id":             {Type: schema.TypeInt, Computed: true, Description: "Identifier of the workflow."},
						"name":           {Type: schema.TypeString, Computed: true, Description: "Name of the workflow."},
						"type":           {Type: schema.TypeString, Computed: true, Description: "Workflow type: `stack` or `edgeStack`."},
						"platform":       {Type: schema.TypeString, Computed: true, Description: "Deployment platform: `dockerStandalone`, `dockerSwarm` or `kubernetes`."},
						"namespace":      {Type: schema.TypeString, Computed: true, Description: "Target Kubernetes namespace, empty for Docker workflows."},
						"endpoint_id":    {Type: schema.TypeInt, Computed: true, Description: "Target environment identifier, zero for an edge-group target."},
						"edge_group_ids": {Type: schema.TypeList, Computed: true, Elem: &schema.Schema{Type: schema.TypeInt}, Description: "Edge group identifiers the workflow targets."},
						"creation_date":  {Type: schema.TypeInt, Computed: true, Description: "Unix timestamp at which the workflow was created."},
						"last_sync_date": {Type: schema.TypeInt, Computed: true, Description: "Unix timestamp of the workflow's last synchronisation."},
					},
				},
			},
		},
	}
}

func dataSourceGitopsSourceRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id := d.Get("source_id").(int)

	var source gitopsSourceDetail
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/gitops/sources/%d", client.Endpoint, id), nil, &source); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read GitOps source %d: %w", id, err))
	}

	var wfList []struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Type     string `json:"type"`
		Platform string `json:"platform"`
		Target   struct {
			Namespace    string `json:"namespace"`
			EndpointID   int    `json:"endpointId"`
			EdgeGroupIDs []int  `json:"edgeGroupIds"`
		} `json:"target"`
		CreationDate int `json:"creationDate"`
		LastSyncDate int `json:"lastSyncDate"`
	}
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/gitops/sources/%d/workflows", client.Endpoint, id), nil, &wfList); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list the workflows of GitOps source %d: %w", id, err))
	}

	workflows := make([]map[string]interface{}, len(wfList))
	for i, w := range wfList {
		groups := w.Target.EdgeGroupIDs
		if groups == nil {
			groups = []int{}
		}
		workflows[i] = map[string]interface{}{
			"id": w.ID, "name": w.Name, "type": w.Type, "platform": w.Platform,
			"namespace": w.Target.Namespace, "endpoint_id": w.Target.EndpointID,
			"edge_group_ids": groups,
			"creation_date":  w.CreationDate, "last_sync_date": w.LastSyncDate,
		}
	}

	teams := source.Access.Teams
	if teams == nil {
		teams = []int{}
	}
	users := source.Access.Users
	if users == nil {
		users = []int{}
	}

	if err := setFields(d, map[string]interface{}{
		"name": source.Name, "url": source.URL, "type": source.Type,
		"status": source.Status, "status_error": source.Error,
		"interval": source.Interval, "last_sync": source.LastSync,
		"username":        source.Connection.Authentication.Username,
		"tls_skip_verify": source.Connection.TLSSkipVerify,
		"public":          source.Access.Public,
		"team_accesses":   teams,
		"user_accesses":   users,
		"workflows":       workflows,
	}); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(strconv.Itoa(id))
	return nil
}
