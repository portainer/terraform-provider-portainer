package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// AlertingRule represents a Portainer alerting rule.
type AlertingRule struct {
	ID                        int               `json:"id,omitempty"`
	Name                      string            `json:"name,omitempty"`
	Description               string            `json:"description,omitempty"`
	Summary                   string            `json:"summary,omitempty"`
	Enabled                   bool              `json:"enabled"`
	Severity                  string            `json:"severity,omitempty"`
	MetricType                string            `json:"metricType,omitempty"`
	ConditionOperator         string            `json:"conditionOperator,omitempty"`
	Threshold                 float64           `json:"threshold,omitempty"`
	Duration                  int               `json:"duration,omitempty"`
	AlertManagerID            int               `json:"alertManagerID,omitempty"`
	IsEditable                bool              `json:"isEditable,omitempty"`
	IsInternal                bool              `json:"isInternal,omitempty"`
	Labels                    map[string]string `json:"labels,omitempty"`
	SupportedAgentVersion     string            `json:"supportedAgentVersion,omitempty"`
	SupportedEnvironmentTypes string            `json:"supportedEnvironmentTypes,omitempty"`
	CreatedAt                 string            `json:"createdAt,omitempty"`
	CreatedBy                 string            `json:"createdBy,omitempty"`
	UpdatedAt                 string            `json:"updatedAt,omitempty"`
}

// AlertRuleUpdatePayload is the body sent for PUT /observability/alerting/rules/{id}.
type AlertRuleUpdatePayload struct {
	AlertingRule AlertingRule `json:"AlertingRule"`
}

func resourceAlertingRule() *schema.Resource {
	return &schema.Resource{
		Create: resourcePortainerAlertingRuleCreate,
		Read:   resourcePortainerAlertingRuleRead,
		Update: resourcePortainerAlertingRuleUpdate,
		Delete: resourcePortainerAlertingRuleDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		Schema: map[string]*schema.Schema{
			"rule_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "The ID of the predefined alert rule to manage. Rules are predefined in Portainer and must be imported by ID.",
			},
			"name": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "Name of the alert rule.",
			},
			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "Description of the alert rule.",
			},
			"summary": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "Summary of the alert rule.",
			},
			"enabled": {
				Type:        schema.TypeBool,
				Required:    true,
				Description: "Whether the alert rule is enabled.",
			},
			"severity": {
				Type:         schema.TypeString,
				Optional:     true,
				Computed:     true,
				Description:  "Severity level of the alert rule.",
				ValidateFunc: validation.StringInSlice([]string{"critical", "warning", "info"}, false),
			},
			"metric_type": {
				Type:         schema.TypeString,
				Optional:     true,
				Computed:     true,
				Description:  "Type of metric for the alert rule.",
				ValidateFunc: validation.StringInSlice([]string{"percentage", "bytes", "raw"}, false),
			},
			"condition_operator": {
				Type:         schema.TypeString,
				Optional:     true,
				Computed:     true,
				Description:  "Condition operator for threshold comparison.",
				ValidateFunc: validation.StringInSlice([]string{">", "<", "=", ">=", "<="}, false),
			},
			"threshold": {
				Type:        schema.TypeFloat,
				Optional:    true,
				Computed:    true,
				Description: "Threshold value for the alert rule.",
			},
			"duration": {
				Type:        schema.TypeInt,
				Optional:    true,
				Computed:    true,
				Description: "Duration (in seconds) the condition must persist before firing.",
			},
			"alert_manager_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Computed:    true,
				Description: "ID of the associated AlertManager settings.",
			},
			"is_editable": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the alert rule can be edited.",
			},
			"is_internal": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the alert rule is an internal/system rule.",
			},
			"labels": {
				Type:        schema.TypeMap,
				Optional:    true,
				Computed:    true,
				Description: "Labels associated with the alert rule.",
				Elem:        &schema.Schema{Type: schema.TypeString},
			},
			"supported_agent_version": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Minimum agent version that supports this rule.",
			},
			"supported_environment_types": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Environment types that support this rule (docker, kubernetes, podman, all).",
			},
			"created_at": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Timestamp when the rule was created.",
			},
			"created_by": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "User who created the rule.",
			},
			"updated_at": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Timestamp when the rule was last updated.",
			},
		},
	}
}

func resourcePortainerAlertingRuleCreate(d *schema.ResourceData, meta interface{}) error {
	// There is no POST endpoint for alerting rules - rules are predefined in Portainer.
	// Create operates by adopting an existing rule via rule_id and applying updates.
	ruleID := d.Get("rule_id").(int)
	d.SetId(strconv.Itoa(ruleID))
	return resourcePortainerAlertingRuleUpdate(d, meta)
}

func resourcePortainerAlertingRuleRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/observability/alerting/rules/%s", client.Endpoint, d.Id()), nil)
	if err != nil {
		return err
	}
	setAlertingAuthHeaders(req, client)

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		d.SetId("")
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to read alert rule %s, status %d: %s", d.Id(), resp.StatusCode, string(body))
	}

	var rule AlertingRule
	if err := json.NewDecoder(resp.Body).Decode(&rule); err != nil {
		return fmt.Errorf("failed to decode alert rule response: %w", err)
	}

	d.SetId(strconv.Itoa(rule.ID))
	_ = d.Set("rule_id", rule.ID)
	_ = d.Set("name", rule.Name)
	_ = d.Set("description", rule.Description)
	_ = d.Set("summary", rule.Summary)
	_ = d.Set("enabled", rule.Enabled)
	_ = d.Set("severity", rule.Severity)
	_ = d.Set("metric_type", rule.MetricType)
	_ = d.Set("condition_operator", rule.ConditionOperator)
	_ = d.Set("threshold", rule.Threshold)
	_ = d.Set("duration", rule.Duration)
	_ = d.Set("alert_manager_id", rule.AlertManagerID)
	_ = d.Set("is_editable", rule.IsEditable)
	_ = d.Set("is_internal", rule.IsInternal)
	_ = d.Set("labels", rule.Labels)
	_ = d.Set("supported_agent_version", rule.SupportedAgentVersion)
	_ = d.Set("supported_environment_types", rule.SupportedEnvironmentTypes)
	_ = d.Set("created_at", rule.CreatedAt)
	_ = d.Set("created_by", rule.CreatedBy)
	_ = d.Set("updated_at", rule.UpdatedAt)

	return nil
}

func resourcePortainerAlertingRuleUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	rule := buildAlertingRulePayload(d)

	jsonPayload, err := json.Marshal(AlertRuleUpdatePayload{AlertingRule: rule})
	if err != nil {
		return fmt.Errorf("failed to marshal alert rule payload: %w", err)
	}

	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/observability/alerting/rules/%s", client.Endpoint, d.Id()), bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}
	setAlertingAuthHeaders(req, client)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update alert rule %s: %s", d.Id(), string(body))
	}

	return resourcePortainerAlertingRuleRead(d, meta)
}

func resourcePortainerAlertingRuleDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/observability/alerting/rules/%s", client.Endpoint, d.Id()), nil)
	if err != nil {
		return err
	}
	setAlertingAuthHeaders(req, client)

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode != 404 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete alert rule %s: %s", d.Id(), string(body))
	}

	d.SetId("")
	return nil
}

func buildAlertingRulePayload(d *schema.ResourceData) AlertingRule {
	rule := AlertingRule{
		Enabled: d.Get("enabled").(bool),
	}

	// Set the ID from resource ID
	if id, err := strconv.Atoi(d.Id()); err == nil {
		rule.ID = id
	}

	if v, ok := d.GetOk("name"); ok {
		rule.Name = v.(string)
	}
	if v, ok := d.GetOk("description"); ok {
		rule.Description = v.(string)
	}
	if v, ok := d.GetOk("summary"); ok {
		rule.Summary = v.(string)
	}
	if v, ok := d.GetOk("severity"); ok {
		rule.Severity = v.(string)
	}
	if v, ok := d.GetOk("metric_type"); ok {
		rule.MetricType = v.(string)
	}
	if v, ok := d.GetOk("condition_operator"); ok {
		rule.ConditionOperator = v.(string)
	}
	if v, ok := d.GetOk("threshold"); ok {
		rule.Threshold = v.(float64)
	}
	if v, ok := d.GetOk("duration"); ok {
		rule.Duration = v.(int)
	}
	if v, ok := d.GetOk("alert_manager_id"); ok {
		rule.AlertManagerID = v.(int)
	}
	if v, ok := d.GetOk("labels"); ok {
		labels := make(map[string]string)
		for k, val := range v.(map[string]interface{}) {
			labels[k] = val.(string)
		}
		rule.Labels = labels
	}

	return rule
}
