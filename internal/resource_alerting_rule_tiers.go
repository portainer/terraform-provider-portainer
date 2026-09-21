package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// alertingRuleTier is one severity step of a multi-severity alert rule.
type alertingRuleTier struct {
	Severity  string  `json:"severity"`
	Threshold float64 `json:"threshold"`
	Enabled   bool    `json:"enabled"`
}

// resourceAlertingRuleTiers configures the multi-severity tiers of an editable
// built-in alert rule: the same metric firing at warning above one threshold
// and critical above another.
//
// Portainer exposes this through its own endpoint because it applies to rules
// Terraform did not create, which is why it is a separate resource rather than
// a block on portainer_alerting_rule.
func resourceAlertingRuleTiers() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAlertingRuleTiersWrite,
		ReadContext:   resourceAlertingRuleTiersRead,
		UpdateContext: resourceAlertingRuleTiersWrite,
		// Portainer has no endpoint to clear a rule's tier configuration.
		// Destroying the resource stops managing it and leaves the rule as
		// configured, rather than guessing at a payload that might disable it.
		DeleteContext: removeFromStateContext,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"rule_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the alert rule to configure. The rule has to be an editable internal rule; Portainer rejects the call for others.",
			},
			"enabled": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "Whether the rule is enabled.",
			},
			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "Description stored on the rule. Leave unset to keep what the rule already has.",
			},
			"condition_operator": {
				Type:         schema.TypeString,
				Optional:     true,
				Computed:     true,
				ValidateFunc: validation.StringInSlice([]string{">", "<", "=", ">=", "<="}, false),
				Description:  "Comparison the thresholds are evaluated with: `>`, `<`, `=`, `>=` or `<=`. Some built-in rules fix the operator and reject a change to it.",
			},
			"duration": {
				Type:        schema.TypeInt,
				Optional:    true,
				Computed:    true,
				Description: "How long the condition has to hold before the rule fires, in the unit the rule itself uses.",
			},
			"tier": {
				Type:     schema.TypeList,
				Required: true,
				// A list rather than a set: the tiers are an ordered ladder of
				// severities, and reordering them is a change worth showing in
				// a plan.
				Description: "Severity tiers of the rule, in order. Each tier fires at its own threshold.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"severity": {
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validation.StringInSlice([]string{"critical", "warning", "info"}, false),
							Description:  "Severity this tier raises: `critical`, `warning` or `info`.",
						},
						"threshold": {
							Type:        schema.TypeFloat,
							Required:    true,
							Description: "Value the metric is compared against for this tier.",
						},
						"enabled": {
							Type:        schema.TypeBool,
							Optional:    true,
							Default:     true,
							Description: "Whether this tier is evaluated.",
						},
					},
				},
			},
			// Computed attributes
			"use_tiers": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether Portainer actually evaluates the rule per tier. A rule that reports `false` here is still using its single threshold.",
			},
		},
	}
}

// alertingTiersFromConfig reads the tier blocks out of the resource data.
func alertingTiersFromConfig(raw interface{}) []alertingRuleTier {
	list, _ := raw.([]interface{})
	tiers := make([]alertingRuleTier, 0, len(list))
	for _, item := range list {
		entry, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		severity, _ := entry["severity"].(string)
		threshold, _ := entry["threshold"].(float64)
		enabled, _ := entry["enabled"].(bool)
		tiers = append(tiers, alertingRuleTier{Severity: severity, Threshold: threshold, Enabled: enabled})
	}
	return tiers
}

func resourceAlertingRuleTiersWrite(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	ruleID := strconv.Itoa(d.Get("rule_id").(int))

	payload := map[string]interface{}{
		"enabled": d.Get("enabled").(bool),
		"tiers":   alertingTiersFromConfig(d.Get("tier")),
	}
	// The three optional fields are omitted while they are still unknown.
	// Portainer keeps the rule's existing value for an omitted field, and a
	// built-in rule's description is not something an unrelated apply should
	// blank on the first write.
	if v, ok := d.GetOk("description"); ok && v.(string) != "" {
		payload["description"] = v.(string)
	}
	if v, ok := d.GetOk("condition_operator"); ok && v.(string) != "" {
		payload["conditionOperator"] = v.(string)
	}
	if v, ok := d.GetOk("duration"); ok && v.(int) != 0 {
		payload["duration"] = v.(int)
	}

	url := fmt.Sprintf("%s/observability/alerting/rules/%s/tiers", client.Endpoint, ruleID)
	if err := doJSON(ctx, client, http.MethodPut, url, payload, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update the tiers of alert rule %s: %w", ruleID, err))
	}

	d.SetId(ruleID)
	return resourceAlertingRuleTiersRead(ctx, d, meta)
}

func resourceAlertingRuleTiersRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	ruleID, err := strconv.Atoi(d.Id())
	if err != nil {
		return diag.FromErr(fmt.Errorf("the resource ID must be an alert rule identifier, got %q: %w", d.Id(), err))
	}

	var rule struct {
		ID                int                `json:"id"`
		Enabled           bool               `json:"enabled"`
		Description       string             `json:"description"`
		ConditionOperator string             `json:"conditionOperator"`
		Duration          int                `json:"duration"`
		UseTiers          bool               `json:"useTiers"`
		Tiers             []alertingRuleTier `json:"tiers"`
	}
	url := fmt.Sprintf("%s/observability/alerting/rules/%d", client.Endpoint, ruleID)
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &rule); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read alert rule %d: %w", ruleID, err))
	}

	tiers := make([]interface{}, 0, len(rule.Tiers))
	for _, tier := range rule.Tiers {
		tiers = append(tiers, map[string]interface{}{
			"severity":  tier.Severity,
			"threshold": tier.Threshold,
			"enabled":   tier.Enabled,
		})
	}

	// description, condition_operator and duration are Optional+Computed, so
	// reading the server's value back into an attribute the configuration left
	// out is what keeps a plan clean rather than what makes it churn.
	fields := map[string]interface{}{
		"rule_id":            ruleID,
		"enabled":            rule.Enabled,
		"tier":               tiers,
		"description":        rule.Description,
		"condition_operator": rule.ConditionOperator,
		"duration":           rule.Duration,
		"use_tiers":          rule.UseTiers,
	}

	if err := setFields(d, fields); err != nil {
		return diag.FromErr(err)
	}
	return nil
}
