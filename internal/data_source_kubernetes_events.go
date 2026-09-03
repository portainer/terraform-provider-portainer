package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceKubernetesEvents() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesEventsRead,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Environment (endpoint) identifier of the Kubernetes environment to query.",
			},
			"namespace": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Namespace to read events from. Leave unset to read events across every namespace the API token can access.",
			},
			"resource_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Only return events for the object with this UID. Cluster-wide queries only.",
			},
			// Computed attributes
			"events": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Events matching the query, as Kubernetes reports them. Events are short-lived — Kubernetes discards them after about an hour by default.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"type":            {Type: schema.TypeString, Computed: true, Description: "Event type, `Normal` or `Warning`."},
						"reason":          {Type: schema.TypeString, Computed: true, Description: "Short machine-readable reason, such as `FailedScheduling`."},
						"message":         {Type: schema.TypeString, Computed: true, Description: "Human-readable description of what happened."},
						"count":           {Type: schema.TypeInt, Computed: true, Description: "How many times this event has occurred."},
						"namespace":       {Type: schema.TypeString, Computed: true, Description: "Namespace the event belongs to."},
						"kind":            {Type: schema.TypeString, Computed: true, Description: "Kind of the object the event is about."},
						"name":            {Type: schema.TypeString, Computed: true, Description: "Name of the object the event is about."},
						"uid":             {Type: schema.TypeString, Computed: true, Description: "UID of the object the event is about."},
						"first_timestamp": {Type: schema.TypeString, Computed: true, Description: "When the event was first recorded."},
						"last_timestamp":  {Type: schema.TypeString, Computed: true, Description: "When the event was last recorded."},
					},
				},
			},
			"warning_count": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of returned events of type `Warning`, which is what usually explains a workload that will not start.",
			},
		},
	}
}

func dataSourceKubernetesEventsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	envID := d.Get("environment_id").(int)
	namespace := d.Get("namespace").(string)

	path := fmt.Sprintf("/kubernetes/%d/events", envID)
	if namespace != "" {
		path = fmt.Sprintf("/kubernetes/%d/namespaces/%s/events", envID, namespace)
	}
	query := ""
	if v, ok := d.GetOk("resource_id"); ok && namespace == "" {
		q := url.Values{}
		q.Set("resourceId", v.(string))
		query = "?" + q.Encode()
	}

	var raw []struct {
		Type           string `json:"type"`
		Reason         string `json:"reason"`
		Message        string `json:"message"`
		Count          int    `json:"count"`
		Namespace      string `json:"namespace"`
		FirstTimestamp string `json:"firstTimestamp"`
		LastTimestamp  string `json:"lastTimestamp"`
		InvolvedObject struct {
			Kind      string `json:"kind"`
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
			UID       string `json:"uid"`
		} `json:"involvedObject"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+path+query, nil, &raw); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read Kubernetes events: %w", err))
	}

	events := make([]map[string]interface{}, len(raw))
	warnings := 0
	for i, e := range raw {
		ns := e.Namespace
		if ns == "" {
			ns = e.InvolvedObject.Namespace
		}
		if e.Type == "Warning" {
			warnings++
		}
		events[i] = map[string]interface{}{
			"type": e.Type, "reason": e.Reason, "message": e.Message, "count": e.Count,
			"namespace": ns, "kind": e.InvolvedObject.Kind, "name": e.InvolvedObject.Name,
			"uid":             e.InvolvedObject.UID,
			"first_timestamp": e.FirstTimestamp, "last_timestamp": e.LastTimestamp,
		}
	}

	if err := setFields(d, map[string]interface{}{
		"events":        events,
		"warning_count": warnings,
	}); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(fmt.Sprintf("%d/%s/events/%s", envID, namespace, strconv.FormatInt(makeTimestamp(), 10)))
	return nil
}
