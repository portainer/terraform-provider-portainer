package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceAlertingConnectivity() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceAlertingConnectivityRead,

		Schema: map[string]*schema.Schema{
			"url": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "URL of the Alertmanager to test.",
			},
			"fail_on_error": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "Whether an unreachable Alertmanager fails the plan. Defaults to `true`, which is the point of a pre-flight check; set it to `false` to branch on `reachable` instead.",
			},
			// Computed attributes
			"reachable": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether Portainer could reach the Alertmanager.",
			},
			"error": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "What went wrong, empty when the Alertmanager answered.",
			},
			"details": {
				Type:     schema.TypeString,
				Computed: true,
				// Portainer does not describe this response's shape in its own
				// specification, so it is carried through as JSON.
				Description: "The whole response as Portainer returns it, encoded as JSON. Portainer does not describe its shape in the API specification, so decode it with `jsondecode()` if you need more than `reachable`.",
			},
		},
	}
}

func dataSourceAlertingConnectivityRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	testURL := fmt.Sprintf("%s/observability/alerting/connectivity?url=%s",
		client.Endpoint, url.QueryEscape(d.Get("url").(string)))

	body, err := apiGETRaw(ctx, client, testURL)
	reachable := err == nil
	message := ""
	if err != nil {
		message = err.Error()
		if d.Get("fail_on_error").(bool) {
			return diag.FromErr(fmt.Errorf("the Alertmanager at %s could not be reached (set fail_on_error = false to read the result instead): %w",
				d.Get("url").(string), err))
		}
		body = nil
	}

	if err := setFields(d, map[string]interface{}{
		"reachable": reachable,
		"error":     message,
		"details":   string(body),
	}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId("portainer-alerting-connectivity-" + d.Get("url").(string))
	return nil
}

func dataSourceEnvironmentLogs() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceEnvironmentLogsRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the environment to query logs for.",
			},
			"from": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Start of the window to query, as Portainer's observability backend expects it (for example an RFC 3339 timestamp).",
			},
			"to": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "End of the window to query.",
			},
			"namespace": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Only return logs from this namespace.",
			},
			"kind": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Only return logs from this resource kind.",
			},
			"name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Only return logs from the resource with this name.",
			},
			"search": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Only return log lines matching this text.",
			},
			"severity": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Only return log lines of this severity.",
			},
			"skip": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Log lines to skip, for paging through a large window.",
			},
			"limit": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Most log lines to return.",
			},
			// Computed attributes
			"logs": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "The log lines the query returned.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"time": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "When the line was logged.",
						},
						"severity": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Severity of the line.",
						},
						"source": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Where the line came from.",
						},
						"message": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The log line itself.",
						},
						"labels": {
							Type:        schema.TypeMap,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Labels attached to the line.",
						},
					},
				},
			},
		},
	}
}

func dataSourceEnvironmentLogsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)

	query := url.Values{}
	query.Set("from", d.Get("from").(string))
	query.Set("to", d.Get("to").(string))
	for field, key := range map[string]string{
		"namespace": "namespace", "kind": "kind", "name": "name",
		"search": "search", "severity": "severity",
	} {
		if v, ok := d.GetOk(field); ok && v.(string) != "" {
			query.Set(key, v.(string))
		}
	}
	for field, key := range map[string]string{"skip": "skip", "limit": "limit"} {
		if v, ok := d.GetOk(field); ok && v.(int) != 0 {
			query.Set(key, fmt.Sprintf("%d", v.(int)))
		}
	}

	var response struct {
		Logs []struct {
			Time     string            `json:"time"`
			Severity string            `json:"severity"`
			Source   string            `json:"source"`
			Message  string            `json:"message"`
			Labels   map[string]string `json:"labels"`
		} `json:"logs"`
	}
	logsURL := fmt.Sprintf("%s/observability/environments/%d/logs?%s", client.Endpoint, endpointID, query.Encode())
	if err := doJSON(ctx, client, http.MethodGet, logsURL, nil, &response); err != nil {
		return diag.FromErr(fmt.Errorf("failed to query the logs of environment %d: %w", endpointID, err))
	}

	entries := make([]interface{}, 0, len(response.Logs))
	for _, entry := range response.Logs {
		entries = append(entries, map[string]interface{}{
			"time": entry.Time, "severity": entry.Severity, "source": entry.Source,
			"message": entry.Message, "labels": stringMap(entry.Labels),
		})
	}

	if err := setFields(d, map[string]interface{}{"logs": entries}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-environment-logs-%d", endpointID))
	return nil
}

func dataSourceEnvironmentMetrics() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceEnvironmentMetricsRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the environment to query metrics for.",
			},
			"metric": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Metric to aggregate.",
			},
			"aggregation": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "How to aggregate it, for example `avg` or `max`.",
			},
			"from": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Start of the window to query.",
			},
			"to": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "End of the window to query.",
			},
			"namespace": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Only aggregate over this namespace.",
			},
			"kind": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Only aggregate over this resource kind.",
			},
			"name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Only aggregate over the resource with this name.",
			},
			"group_by": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Dimension to group the result by.",
			},
			// Computed attributes
			"data": {
				Type:     schema.TypeString,
				Computed: true,
				// The series' shape depends on the metric and the grouping, so
				// it is carried through as JSON rather than flattened into
				// attributes that would only fit one kind of query.
				Description: "The aggregated series as Portainer returns them, encoded as JSON. Decode it with `jsondecode()`; the shape depends on `metric` and `group_by`.",
			},
		},
	}
}

func dataSourceEnvironmentMetricsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)

	query := url.Values{}
	query.Set("metric", d.Get("metric").(string))
	query.Set("aggregation", d.Get("aggregation").(string))
	query.Set("from", d.Get("from").(string))
	query.Set("to", d.Get("to").(string))
	for field, key := range map[string]string{
		"namespace": "namespace", "kind": "kind", "name": "name", "group_by": "groupBy",
	} {
		if v, ok := d.GetOk(field); ok && v.(string) != "" {
			query.Set(key, v.(string))
		}
	}

	var response struct {
		Data json.RawMessage `json:"data"`
	}
	metricsURL := fmt.Sprintf("%s/observability/environments/%d/metrics?%s", client.Endpoint, endpointID, query.Encode())
	if err := doJSON(ctx, client, http.MethodGet, metricsURL, nil, &response); err != nil {
		return diag.FromErr(fmt.Errorf("failed to query the metrics of environment %d: %w", endpointID, err))
	}

	data := "[]"
	if len(response.Data) > 0 {
		data = string(response.Data)
	}

	if err := setFields(d, map[string]interface{}{"data": data}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-environment-metrics-%d-%s", endpointID, d.Get("metric").(string)))
	return nil
}
