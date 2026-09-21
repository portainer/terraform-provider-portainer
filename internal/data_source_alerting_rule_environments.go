package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// dataSourceAlertingRuleEnvironments reports where an alert rule actually
// landed. Attaching a rule to an endpoint group is not the same as the rule
// being evaluated there - an agent can be too old, an environment the wrong
// type - so the delivery status is what tells an operator whether the
// attachment did anything.
func dataSourceAlertingRuleEnvironments() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceAlertingRuleEnvironmentsRead,

		Schema: map[string]*schema.Schema{
			"rule_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the alert rule to report on.",
			},
			// Computed attributes
			"total": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of environments the rule reaches, across every status below.",
			},
			"active": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of environments where the rule is being evaluated.",
			},
			"pending": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of environments where the rule has not been delivered yet.",
			},
			"error": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of environments where delivering the rule failed. A non-zero value is the one worth alerting on.",
			},
			"excluded": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of environments deliberately left out of the rule.",
			},
			"not_supported": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of environments that cannot run the rule, typically an agent too old or an environment of the wrong type.",
			},
			"groups": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Delivery status broken down by endpoint group.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"endpoint_group_id": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Identifier of the endpoint group.",
						},
						"endpoint_group_name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Name of the endpoint group.",
						},
						"size": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Number of environments in the group.",
						},
						"environments": {
							Type:        schema.TypeList,
							Computed:    true,
							Description: "The environments in the group and how the rule fared on each.",
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"endpoint_id": {
										Type:        schema.TypeInt,
										Computed:    true,
										Description: "Identifier of the environment.",
									},
									"name": {
										Type:        schema.TypeString,
										Computed:    true,
										Description: "Name of the environment.",
									},
									"status": {
										Type:        schema.TypeString,
										Computed:    true,
										Description: "Delivery status of the rule on this environment.",
									},
									"reason_code": {
										Type:        schema.TypeString,
										Computed:    true,
										Description: "Machine-readable reason behind the status, which is what to branch on rather than the message.",
									},
									"message": {
										Type:        schema.TypeString,
										Computed:    true,
										Description: "Human-readable explanation of the status.",
									},
									"updated_at": {
										Type:        schema.TypeInt,
										Computed:    true,
										Description: "Unix timestamp of the last status change.",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func dataSourceAlertingRuleEnvironmentsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	ruleID := d.Get("rule_id").(int)

	var response struct {
		Total        int `json:"total"`
		Active       int `json:"active"`
		Pending      int `json:"pending"`
		Error        int `json:"error"`
		Excluded     int `json:"excluded"`
		NotSupported int `json:"notSupported"`
		Groups       []struct {
			EndpointGroupID   int    `json:"endpointGroupId"`
			EndpointGroupName string `json:"endpointGroupName"`
			Size              int    `json:"size"`
			Environments      []struct {
				EndpointID int    `json:"endpointId"`
				Name       string `json:"name"`
				Status     string `json:"status"`
				ReasonCode string `json:"reasonCode"`
				Message    string `json:"message"`
				UpdatedAt  int64  `json:"updatedAt"`
			} `json:"environments"`
		} `json:"groups"`
	}

	url := fmt.Sprintf("%s/observability/alerting/rules/%d/environments", client.Endpoint, ruleID)
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &response); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the environments of alert rule %d: %w", ruleID, err))
	}

	groups := make([]interface{}, 0, len(response.Groups))
	for _, group := range response.Groups {
		environments := make([]interface{}, 0, len(group.Environments))
		for _, environment := range group.Environments {
			environments = append(environments, map[string]interface{}{
				"endpoint_id": environment.EndpointID,
				"name":        environment.Name,
				"status":      environment.Status,
				"reason_code": environment.ReasonCode,
				"message":     environment.Message,
				"updated_at":  int(environment.UpdatedAt),
			})
		}
		groups = append(groups, map[string]interface{}{
			"endpoint_group_id":   group.EndpointGroupID,
			"endpoint_group_name": group.EndpointGroupName,
			"size":                group.Size,
			"environments":        environments,
		})
	}

	if err := setFields(d, map[string]interface{}{
		"total":         response.Total,
		"active":        response.Active,
		"pending":       response.Pending,
		"error":         response.Error,
		"excluded":      response.Excluded,
		"not_supported": response.NotSupported,
		"groups":        groups,
	}); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(fmt.Sprintf("portainer-alerting-rule-environments-%d", ruleID))
	return nil
}
