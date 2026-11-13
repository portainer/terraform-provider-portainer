package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// AlertingRule represents a Portainer alerting rule (portaineree.AlertingRule).
// It is only decoded; request bodies are built from the configured
// attributes, see alertingRulePayload.
type AlertingRule struct {
	ID                        int                `json:"id,omitempty"`
	Name                      string             `json:"name,omitempty"`
	Description               string             `json:"description,omitempty"`
	Summary                   string             `json:"summary,omitempty"`
	Category                  string             `json:"category,omitempty"`
	Enabled                   bool               `json:"enabled"`
	Severity                  string             `json:"severity,omitempty"`
	MetricType                string             `json:"metricType,omitempty"`
	ConditionOperator         string             `json:"conditionOperator,omitempty"`
	OperatorFixed             bool               `json:"operatorFixed,omitempty"`
	Threshold                 float64            `json:"threshold,omitempty"`
	Duration                  int                `json:"duration,omitempty"`
	ForDurationMinutes        int                `json:"forDurationMinutes,omitempty"`
	UseDurationAsFor          bool               `json:"useDurationAsFor,omitempty"`
	AlertManagerID            int                `json:"alertManagerID,omitempty"`
	EndpointGroupIDs          []int              `json:"endpointGroupIds,omitempty"`
	TagIDs                    []int              `json:"tagIds,omitempty"`
	PromqlExpr                string             `json:"promqlExpr,omitempty"`
	PromqlTemplate            string             `json:"promqlTemplate,omitempty"`
	PromqlScalarTemplate      string             `json:"promqlScalarTemplate,omitempty"`
	AnnotationTemplate        string             `json:"annotationTemplate,omitempty"`
	UseTiers                  bool               `json:"useTiers,omitempty"`
	IndependentTiers          bool               `json:"independentTiers,omitempty"`
	Tiers                     []alertingRuleTier `json:"tiers,omitempty"`
	IsEditable                bool               `json:"isEditable,omitempty"`
	IsInternal                bool               `json:"isInternal,omitempty"`
	Labels                    map[string]string  `json:"labels,omitempty"`
	SupportedAgentVersion     string             `json:"supportedAgentVersion,omitempty"`
	SupportedEnvironmentTypes string             `json:"supportedEnvironmentTypes,omitempty"`
	CreatedAt                 string             `json:"createdAt,omitempty"`
	CreatedBy                 string             `json:"createdBy,omitempty"`
	UpdatedAt                 string             `json:"updatedAt,omitempty"`
}

// alertingRuleWritable maps every attribute a configuration can set to its
// JSON name in portaineree.AlertingRule.
var alertingRuleWritable = []struct{ key, json string }{
	{"name", "name"},
	{"description", "description"},
	{"summary", "summary"},
	{"category", "category"},
	{"severity", "severity"},
	{"metric_type", "metricType"},
	{"condition_operator", "conditionOperator"},
	{"threshold", "threshold"},
	{"duration", "duration"},
	{"for_duration_minutes", "forDurationMinutes"},
	{"alert_manager_id", "alertManagerID"},
	{"endpoint_group_ids", "endpointGroupIds"},
	{"tag_ids", "tagIds"},
	{"promql_expr", "promqlExpr"},
	{"supported_environment_types", "supportedEnvironmentTypes"},
	{"labels", "labels"},
}

func resourceAlertingRule() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePortainerAlertingRuleCreate,
		ReadContext:   resourcePortainerAlertingRuleRead,
		UpdateContext: resourcePortainerAlertingRuleUpdate,
		DeleteContext: resourcePortainerAlertingRuleDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"rule_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Computed:    true,
				ForceNew:    true,
				Description: "ID of the alert rule. Set it to adopt an existing rule (typically one of Portainer's predefined rules); leave it unset to create a custom rule, whose ID is then reported here.",
			},
			"name": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "Name of the alert rule. Required when creating a custom rule (`rule_id` unset).",
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
			"category": {
				Type:         schema.TypeString,
				Optional:     true,
				Computed:     true,
				Description:  "Category the rule is grouped under: `portainer`, `security`, `environment`, `backup` or `addons` (`backup` and `addons` need Portainer BE 3.0 or later).",
				ValidateFunc: validation.StringInSlice([]string{"portainer", "security", "environment", "backup", "addons"}, false),
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
				Description:  "Severity level of the alert rule: `critical`, `warning` or `info`.",
				ValidateFunc: validation.StringInSlice([]string{"critical", "warning", "info"}, false),
			},
			"metric_type": {
				Type:         schema.TypeString,
				Optional:     true,
				Computed:     true,
				Description:  "Unit of the metric the threshold is compared against: `percentage`, `bytes`, `raw` or `days`.",
				ValidateFunc: validation.StringInSlice([]string{"percentage", "bytes", "raw", "days"}, false),
			},
			"condition_operator": {
				Type:         schema.TypeString,
				Optional:     true,
				Computed:     true,
				Description:  "Condition operator for threshold comparison: `>`, `<`, `=`, `>=` or `<=`. A rule with `operator_fixed` rejects a change.",
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
				Description: "Period within which the condition and threshold must hold for the rule to fire, in the unit the rule itself uses.",
			},
			"for_duration_minutes": {
				Type:        schema.TypeInt,
				Optional:    true,
				Computed:    true,
				Description: "Minutes the condition has to keep holding before the alert fires (the `for` clause of the generated Prometheus rule).",
			},
			"alert_manager_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Computed:    true,
				Description: "ID of the associated AlertManager settings.",
			},
			"endpoint_group_ids": {
				Type:        schema.TypeList,
				Optional:    true,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "IDs of the endpoint groups the rule is restricted to; empty means every relevant environment. Do not also manage the same rule with `portainer_alerting_rule_groups`.",
			},
			"tag_ids": {
				Type:        schema.TypeList,
				Optional:    true,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "IDs of the environment tags the rule is restricted to.",
			},
			"promql_expr": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "PromQL expression the rule evaluates; this is what a custom rule measures.",
			},
			"supported_environment_types": {
				Type:         schema.TypeString,
				Optional:     true,
				Computed:     true,
				Description:  "Environment types the rule applies to: `docker`, `kubernetes`, `podman` or `all`.",
				ValidateFunc: validation.StringInSlice([]string{"docker", "kubernetes", "podman", "all"}, false),
			},
			"labels": {
				Type:        schema.TypeMap,
				Optional:    true,
				Computed:    true,
				Description: "Labels associated with the alert rule.",
				Elem:        &schema.Schema{Type: schema.TypeString},
			},
			// Computed attributes
			"created": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether Terraform created this rule. Only a created rule is deleted from Portainer on destroy; an adopted or imported rule is only removed from the state.",
			},
			"is_editable": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the alert rule can be edited.",
			},
			"is_internal": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the alert rule is one of Portainer's built-in rules.",
			},
			"operator_fixed": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the rule's definition fixes its condition operator.",
			},
			"use_duration_as_for": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the rule uses `duration` as the `for` clause of its Prometheus rule.",
			},
			"use_tiers": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the rule is evaluated per severity tier rather than against its single threshold.",
			},
			"independent_tiers": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether each tier measures its own series (Portainer BE 3.0 or later; always false before).",
			},
			"tiers": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Severity tiers of the rule. Configure them with `portainer_alerting_rule_tiers`.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"severity": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Severity this tier raises.",
						},
						"threshold": {
							Type:        schema.TypeFloat,
							Computed:    true,
							Description: "Value the metric is compared against for this tier.",
						},
						"enabled": {
							Type:        schema.TypeBool,
							Computed:    true,
							Description: "Whether this tier is evaluated.",
						},
						"selector": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Series selector this tier measures when `independent_tiers` is true (Portainer BE 3.0 or later).",
						},
					},
				},
			},
			"promql_template": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "PromQL template of a built-in rule, part of the rule's definition.",
			},
			"promql_scalar_template": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Scalar PromQL template of a built-in rule, part of the rule's definition.",
			},
			"annotation_template": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Template of the annotation the rule's alerts carry, part of the rule's definition.",
			},
			"supported_agent_version": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Minimum agent version that supports this rule.",
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

func resourcePortainerAlertingRuleCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// With rule_id the resource adopts an existing rule - most often one of
	// the rules Portainer predefines, which cannot be created - and applies
	// the configuration to it with the update call.
	if ruleID, ok := d.GetOk("rule_id"); ok {
		d.SetId(strconv.Itoa(ruleID.(int)))
		return resourcePortainerAlertingRuleUpdate(ctx, d, meta)
	}

	client := meta.(*APIClient)
	name := d.Get("name").(string)
	if name == "" {
		return diag.Errorf("name is required to create a custom alert rule; set rule_id instead to adopt an existing rule")
	}

	var created AlertingRule
	url := client.Endpoint + "/observability/alerting/rules"
	if err := doJSON(ctx, client, http.MethodPost, url, map[string]interface{}{"AlertingRule": alertingRulePayload(d)}, &created); err != nil {
		return diag.FromErr(fmt.Errorf("failed to create alert rule %q: %w", name, err))
	}

	// The spec types the response as a bare object. Should it not carry the
	// new rule's ID, the rule is looked up by name instead.
	if created.ID == 0 {
		id, err := findAlertingRuleByName(ctx, client, name)
		if err != nil {
			// The rule exists but its ID is unknown, so it cannot go into
			// state; say so, or the next apply silently creates a second one.
			return diag.Errorf("alert rule %q was created, but its ID could not be determined (%v); import it with `terraform import` using its ID from the Portainer UI before applying again", name, err)
		}
		created.ID = id
	}

	d.SetId(strconv.Itoa(created.ID))
	if err := d.Set("created", true); err != nil {
		return diag.FromErr(err)
	}
	return resourcePortainerAlertingRuleRead(ctx, d, meta)
}

// findAlertingRuleByName returns the ID of the custom rule called name. Rule
// names are not unique, so of several matches the newest (highest ID) wins:
// that is the one the create call just added.
func findAlertingRuleByName(ctx context.Context, client *APIClient, name string) (int, error) {
	var rules []AlertingRule
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/observability/alerting/rules", nil, &rules); err != nil {
		return 0, fmt.Errorf("alert rule %q was created, but listing the rules to find its ID failed: %w", name, err)
	}
	id := 0
	for _, rule := range rules {
		if rule.Name == name && !rule.IsInternal && rule.ID > id {
			id = rule.ID
		}
	}
	if id == 0 {
		return 0, fmt.Errorf("alert rule %q was created, but it is not in the list of alert rules", name)
	}
	return id, nil
}

func resourcePortainerAlertingRuleRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	var rule AlertingRule
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/observability/alerting/rules/%s", client.Endpoint, d.Id()), nil, &rule); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read alert rule %s: %w", d.Id(), err))
	}

	d.SetId(strconv.Itoa(rule.ID))
	// created is not part of the rule: it stays what Create set, and false
	// after an import or an adoption.
	if err := setFields(d, alertingRuleFields(rule)); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

// alertingRuleFields flattens a rule into the attributes the resource and the
// portainer_alerting_rules data source share.
func alertingRuleFields(rule AlertingRule) map[string]interface{} {
	tiers := make([]interface{}, 0, len(rule.Tiers))
	for _, tier := range rule.Tiers {
		tiers = append(tiers, map[string]interface{}{
			"severity":  tier.Severity,
			"threshold": tier.Threshold,
			"enabled":   tier.Enabled,
			"selector":  tier.Selector,
		})
	}
	endpointGroupIDs := rule.EndpointGroupIDs
	if endpointGroupIDs == nil {
		endpointGroupIDs = []int{}
	}
	tagIDs := rule.TagIDs
	if tagIDs == nil {
		tagIDs = []int{}
	}

	return map[string]interface{}{
		"rule_id":                     rule.ID,
		"name":                        rule.Name,
		"description":                 rule.Description,
		"summary":                     rule.Summary,
		"category":                    rule.Category,
		"enabled":                     rule.Enabled,
		"severity":                    rule.Severity,
		"metric_type":                 rule.MetricType,
		"condition_operator":          rule.ConditionOperator,
		"operator_fixed":              rule.OperatorFixed,
		"threshold":                   rule.Threshold,
		"duration":                    rule.Duration,
		"for_duration_minutes":        rule.ForDurationMinutes,
		"use_duration_as_for":         rule.UseDurationAsFor,
		"alert_manager_id":            rule.AlertManagerID,
		"endpoint_group_ids":          endpointGroupIDs,
		"tag_ids":                     tagIDs,
		"promql_expr":                 rule.PromqlExpr,
		"promql_template":             rule.PromqlTemplate,
		"promql_scalar_template":      rule.PromqlScalarTemplate,
		"annotation_template":         rule.AnnotationTemplate,
		"use_tiers":                   rule.UseTiers,
		"independent_tiers":           rule.IndependentTiers,
		"tiers":                       tiers,
		"is_editable":                 rule.IsEditable,
		"is_internal":                 rule.IsInternal,
		"labels":                      rule.Labels,
		"supported_agent_version":     rule.SupportedAgentVersion,
		"supported_environment_types": rule.SupportedEnvironmentTypes,
		"created_at":                  rule.CreatedAt,
		"created_by":                  rule.CreatedBy,
		"updated_at":                  rule.UpdatedAt,
	}
}

func resourcePortainerAlertingRuleUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	url := fmt.Sprintf("%s/observability/alerting/rules/%s", client.Endpoint, d.Id())

	// Whether the PUT merges or replaces the rule is not documented. The rule
	// is read first and sent back whole with the configuration laid over it,
	// so a predefined rule keeps its PromQL, templates and tiers either way.
	rule := map[string]interface{}{}
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &rule); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read alert rule %s before updating it: %w", d.Id(), err))
	}
	// A tier's selector is part of the built-in definition and never accepted
	// from a client payload (3.0 spec), so it is not sent back.
	if tiers, ok := rule["tiers"].([]interface{}); ok {
		for _, tier := range tiers {
			if m, ok := tier.(map[string]interface{}); ok {
				delete(m, "selector")
			}
		}
	}
	for k, v := range alertingRulePayload(d) {
		rule[k] = v
	}
	if id, err := strconv.Atoi(d.Id()); err == nil {
		rule["id"] = id
	}

	if err := doJSON(ctx, client, http.MethodPut, url, map[string]interface{}{"AlertingRule": rule}, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update alert rule %s: %w", d.Id(), err))
	}

	return resourcePortainerAlertingRuleRead(ctx, d, meta)
}

func resourcePortainerAlertingRuleDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// A rule Terraform did not create - a predefined rule it adopted, or one
	// it imported - outlives the resource: destroying it only stops managing
	// it.
	if !d.Get("created").(bool) {
		d.SetId("")
		return nil
	}

	client := meta.(*APIClient)
	if err := doJSON(ctx, client, http.MethodDelete, fmt.Sprintf("%s/observability/alerting/rules/%s", client.Endpoint, d.Id()), nil, nil); err != nil {
		if !isAPINotFound(err) {
			return diag.FromErr(fmt.Errorf("failed to delete alert rule %s: %w", d.Id(), err))
		}
	}

	d.SetId("")
	return nil
}

// alertingRulePayload returns enabled plus the attributes the configuration
// sets: the whole body of a create, and what an update lays over the rule as
// Portainer returns it. An attribute the configuration never mentions is
// therefore never handed to Portainer as a zero value.
func alertingRulePayload(d *schema.ResourceData) map[string]interface{} {
	rule := map[string]interface{}{
		"enabled": d.Get("enabled").(bool),
	}
	for _, f := range alertingRuleWritable {
		if !alertingRuleConfigured(d, f.key) {
			continue
		}
		switch v := d.Get(f.key).(type) {
		case []interface{}:
			rule[f.json] = toIntSlice(v)
		case map[string]interface{}:
			labels := make(map[string]string, len(v))
			for k, val := range v {
				labels[k], _ = val.(string)
			}
			rule[f.json] = labels
		default:
			rule[f.json] = v
		}
	}
	return rule
}

// alertingRuleConfigured reports whether the configuration sets key. Without
// a configuration to look at (unit tests) a non-zero value counts as set.
func alertingRuleConfigured(d *schema.ResourceData, key string) bool {
	if d.GetRawConfig().IsNull() {
		_, ok := d.GetOk(key)
		return ok
	}
	raw, diags := d.GetRawConfigAt(cty.GetAttrPath(key))
	return !diags.HasError() && raw.IsKnown() && !raw.IsNull()
}
