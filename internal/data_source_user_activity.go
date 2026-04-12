package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceUserActivity() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceUserActivityRead,

		Schema: map[string]*schema.Schema{
			"log_type": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "activity",
				Description: "Type of logs to retrieve: 'activity' for user activity logs, 'auth' for authentication logs.",
			},
			"keyword": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Filter logs by keyword.",
			},
			"username": {
				Type:        schema.TypeList,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Filter by usernames (only for activity logs).",
			},
			"context": {
				Type:        schema.TypeList,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Filter by context (only for activity logs).",
			},
			"before": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Return results before this unix timestamp.",
			},
			"after": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Return results after this unix timestamp.",
			},
			"sort_by": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Sort results by this column.",
			},
			"sort_desc": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Sort in descending order.",
			},
			"offset": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     0,
				Description: "Pagination offset.",
			},
			"limit": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     100,
				Description: "Limit number of results.",
			},
			// Computed
			"activity_logs": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Type:     schema.TypeInt,
							Computed: true,
						},
						"timestamp": {
							Type:     schema.TypeInt,
							Computed: true,
						},
						"username": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"action": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"context": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
			"auth_logs": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Type:     schema.TypeInt,
							Computed: true,
						},
						"timestamp": {
							Type:     schema.TypeInt,
							Computed: true,
						},
						"username": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"type": {
							Type:     schema.TypeInt,
							Computed: true,
						},
						"origin": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"context": {
							Type:     schema.TypeInt,
							Computed: true,
						},
					},
				},
			},
			"total_count": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Total count of matching logs (only for activity logs).",
			},
		},
	}
}

func dataSourceUserActivityRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	logType := d.Get("log_type").(string)

	var path string
	switch logType {
	case "activity":
		path = "/useractivity/logs"
	case "auth":
		path = "/useractivity/authlogs"
	default:
		return fmt.Errorf("invalid log_type %q: must be 'activity' or 'auth'", logType)
	}

	// Build query parameters
	params := []string{}

	if v, ok := d.GetOk("offset"); ok {
		params = append(params, fmt.Sprintf("offset=%d", v.(int)))
	}
	if v, ok := d.GetOk("limit"); ok {
		params = append(params, fmt.Sprintf("limit=%d", v.(int)))
	}
	if v, ok := d.GetOk("before"); ok {
		params = append(params, fmt.Sprintf("before=%d", v.(int)))
	}
	if v, ok := d.GetOk("after"); ok {
		params = append(params, fmt.Sprintf("after=%d", v.(int)))
	}
	if v, ok := d.GetOk("sort_by"); ok {
		params = append(params, fmt.Sprintf("sortBy=%s", url.QueryEscape(v.(string))))
	}
	if v := d.Get("sort_desc").(bool); v {
		params = append(params, "sortDesc=true")
	}
	if v, ok := d.GetOk("keyword"); ok {
		params = append(params, fmt.Sprintf("keyword=%s", url.QueryEscape(v.(string))))
	}

	// Activity-log-only filters
	if logType == "activity" {
		if v, ok := d.GetOk("username"); ok {
			usernames := v.([]interface{})
			vals := make([]string, len(usernames))
			for i, u := range usernames {
				vals[i] = u.(string)
			}
			params = append(params, fmt.Sprintf("username=%s", url.QueryEscape(strings.Join(vals, ","))))
		}
		if v, ok := d.GetOk("context"); ok {
			contexts := v.([]interface{})
			vals := make([]string, len(contexts))
			for i, c := range contexts {
				vals[i] = c.(string)
			}
			params = append(params, fmt.Sprintf("context=%s", url.QueryEscape(strings.Join(vals, ","))))
		}
	}

	if len(params) > 0 {
		path = path + "?" + strings.Join(params, "&")
	}

	resp, err := client.DoRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to list user activity logs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to list user activity logs: HTTP %d", resp.StatusCode)
	}

	if logType == "activity" {
		var result struct {
			Logs []struct {
				ID        int    `json:"id"`
				Timestamp int    `json:"timestamp"`
				Username  string `json:"username"`
				Action    string `json:"action"`
				Context   string `json:"context"`
			} `json:"logs"`
			TotalCount int `json:"totalCount"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return fmt.Errorf("failed to decode activity logs response: %w", err)
		}

		logs := make([]map[string]interface{}, len(result.Logs))
		for i, l := range result.Logs {
			logs[i] = map[string]interface{}{
				"id":        l.ID,
				"timestamp": l.Timestamp,
				"username":  l.Username,
				"action":    l.Action,
				"context":   l.Context,
			}
		}
		_ = d.Set("activity_logs", logs)
		_ = d.Set("total_count", result.TotalCount)
	} else {
		var result []struct {
			ID        int    `json:"id"`
			Timestamp int    `json:"timestamp"`
			Username  string `json:"username"`
			Type      int    `json:"type"`
			Origin    string `json:"origin"`
			Context   int    `json:"context"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return fmt.Errorf("failed to decode auth logs response: %w", err)
		}

		logs := make([]map[string]interface{}, len(result))
		for i, l := range result {
			logs[i] = map[string]interface{}{
				"id":        l.ID,
				"timestamp": l.Timestamp,
				"username":  l.Username,
				"type":      l.Type,
				"origin":    l.Origin,
				"context":   l.Context,
			}
		}
		_ = d.Set("auth_logs", logs)
	}

	d.SetId(strconv.FormatInt(time.Now().Unix(), 10))
	return nil
}
