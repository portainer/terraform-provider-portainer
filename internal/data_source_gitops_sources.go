package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// gitopsStatusSummarySchema is the workflows.StatusSummary object, returned by
// both the source and the workflow summary endpoints.
func gitopsStatusSummarySchema(what string) *schema.Schema {
	return &schema.Schema{
		Type:        schema.TypeList,
		Computed:    true,
		Description: "Count of " + what + " per sync status, as reported by Portainer's summary endpoint. Always exactly one element.",
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"healthy": {Type: schema.TypeInt, Computed: true, Description: "Number of " + what + " in the `healthy` status."},
				"syncing": {Type: schema.TypeInt, Computed: true, Description: "Number of " + what + " currently syncing."},
				"error":   {Type: schema.TypeInt, Computed: true, Description: "Number of " + what + " whose last sync failed."},
				"paused":  {Type: schema.TypeInt, Computed: true, Description: "Number of " + what + " whose syncing is paused."},
				"unknown": {Type: schema.TypeInt, Computed: true, Description: "Number of " + what + " in an unknown status."},
			},
		},
	}
}

type gitopsStatusSummary struct {
	Healthy int `json:"healthy"`
	Syncing int `json:"syncing"`
	Error   int `json:"error"`
	Paused  int `json:"paused"`
	Unknown int `json:"unknown"`
}

func (s gitopsStatusSummary) toList() []map[string]interface{} {
	return []map[string]interface{}{{
		"healthy": s.Healthy, "syncing": s.Syncing, "error": s.Error,
		"paused": s.Paused, "unknown": s.Unknown,
	}}
}

func dataSourceGitopsSources() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceGitopsSourcesRead,

		Schema: map[string]*schema.Schema{
			"search": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Free-text filter matched against the source name and URL.",
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
			"status": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringInSlice([]string{"healthy", "syncing", "error", "paused", "unknown"}, false),
				Description:  "Only return sources in this sync status.",
			},
			"type": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringInSlice([]string{"git", "helm", "oci"}, false),
				Description:  "Only return sources of this type.",
			},
			// Computed attributes
			"sources": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "GitOps sources matching the query.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id":           {Type: schema.TypeInt, Computed: true, Description: "Identifier of the source."},
						"name":         {Type: schema.TypeString, Computed: true, Description: "Name of the source."},
						"url":          {Type: schema.TypeString, Computed: true, Description: "Repository URL the source syncs from."},
						"type":         {Type: schema.TypeString, Computed: true, Description: "Source type: `git`, `helm` or `oci`."},
						"status":       {Type: schema.TypeString, Computed: true, Description: "Sync status: `healthy`, `syncing`, `error`, `paused` or `unknown`."},
						"status_error": {Type: schema.TypeString, Computed: true, Description: "Error reported by the last synchronisation attempt, empty while healthy."},
						"interval":     {Type: schema.TypeString, Computed: true, Description: "Polling interval of the source, as a duration string."},
						"last_sync":    {Type: schema.TypeInt, Computed: true, Description: "Unix timestamp of the last successful synchronisation."},
						"used_by":      {Type: schema.TypeInt, Computed: true, Description: "Number of workflows using this source."},
						"environments": {Type: schema.TypeInt, Computed: true, Description: "Number of environments the source's workflows deploy to."},
					},
				},
			},
			"summary": gitopsStatusSummarySchema("GitOps sources"),
		},
	}
}

func dataSourceGitopsSourcesRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	q := url.Values{}
	for attr, param := range map[string]string{
		"search": "search", "sort": "sort", "order": "order", "status": "status", "type": "type",
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
	query := ""
	if len(q) > 0 {
		query = "?" + q.Encode()
	}

	var list []struct {
		ID           int    `json:"id"`
		Name         string `json:"name"`
		URL          string `json:"url"`
		Type         string `json:"type"`
		Status       string `json:"status"`
		Error        string `json:"error"`
		Interval     string `json:"interval"`
		LastSync     int    `json:"lastSync"`
		UsedBy       int    `json:"usedBy"`
		Environments int    `json:"environments"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/gitops/sources"+query, nil, &list); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list GitOps sources: %w", err))
	}

	sources := make([]map[string]interface{}, len(list))
	for i, s := range list {
		sources[i] = map[string]interface{}{
			"id": s.ID, "name": s.Name, "url": s.URL, "type": s.Type,
			"status": s.Status, "status_error": s.Error, "interval": s.Interval,
			"last_sync": s.LastSync, "used_by": s.UsedBy, "environments": s.Environments,
		}
	}

	// The summary is its own endpoint and is unfiltered: it always describes
	// every source, not just the ones matching the query above.
	var summary gitopsStatusSummary
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/gitops/sources/summary", nil, &summary); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the GitOps source summary: %w", err))
	}

	if err := setFields(d, map[string]interface{}{
		"sources": sources,
		"summary": summary.toList(),
	}); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("gitops-sources-" + strconv.FormatInt(makeTimestamp(), 10))
	return nil
}
