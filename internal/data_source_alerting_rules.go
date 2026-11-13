package internal

import (
	"context"
	"fmt"
	"net/http"
	"sort"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// dataSourceAlertingRules lists the alert rules, predefined and custom. Its
// main use is finding the ID of a predefined rule by name, so that
// portainer_alerting_rule can adopt it without a hard-coded rule_id: IDs are
// assigned per Portainer instance.
func dataSourceAlertingRules() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceAlertingRulesRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Only list the rules with exactly this name.",
			},
			"category": {
				Type:         schema.TypeString,
				Optional:     true,
				Description:  "Only list the rules of this category: `portainer`, `security`, `environment`, `backup` or `addons`.",
				ValidateFunc: validation.StringInSlice([]string{"portainer", "security", "environment", "backup", "addons"}, false),
			},
			// Computed attributes
			"rules": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "The matching alert rules, ordered by ID. Each element has the attributes of `portainer_alerting_rule` except `created`.",
				Elem:        &schema.Resource{Schema: alertingRuleListElemSchema()},
			},
		},
	}
}

// alertingRuleListElemSchema is the schema of portainer_alerting_rule turned
// read-only, so the list element and the resource cannot drift apart.
func alertingRuleListElemSchema() map[string]*schema.Schema {
	out := make(map[string]*schema.Schema)
	for key, s := range resourceAlertingRule().Schema {
		if key == "created" {
			continue
		}
		c := *s
		c.Optional, c.Required, c.ForceNew, c.ValidateFunc = false, false, false, nil
		c.Computed = true
		out[key] = &c
	}
	out["rule_id"].Description = "ID of the alert rule; pass it to `portainer_alerting_rule` to adopt the rule."
	return out
}

func dataSourceAlertingRulesRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	var rules []AlertingRule
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/observability/alerting/rules", nil, &rules); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list alert rules: %w", err))
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].ID < rules[j].ID })

	name := d.Get("name").(string)
	category := d.Get("category").(string)
	out := make([]interface{}, 0, len(rules))
	for _, rule := range rules {
		if (name != "" && rule.Name != name) || (category != "" && rule.Category != category) {
			continue
		}
		out = append(out, alertingRuleFields(rule))
	}

	if err := d.Set("rules", out); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-alerting-rules-%s-%s", category, name))
	return nil
}
