package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// resourceAlertingRuleGroups attaches an alert rule to endpoint groups.
//
// It is a resource of its own rather than a field on portainer_alerting_rule
// because most rules worth attaching are Portainer's own built-in ones: they
// are not created by Terraform and cannot be, but which environments they
// cover is exactly what an operator wants under version control.
func resourceAlertingRuleGroups() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAlertingRuleGroupsWrite,
		ReadContext:   resourceAlertingRuleGroupsRead,
		UpdateContext: resourceAlertingRuleGroupsWrite,
		DeleteContext: resourceAlertingRuleGroupsDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"rule_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the alert rule whose endpoint group attachments this resource owns. Built-in rules can be attached the same way rules created by `portainer_alerting_rule` can.",
			},
			"endpoint_group_ids": {
				Type:        schema.TypeList,
				Required:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "Identifiers of the endpoint groups the rule applies to. The list is authoritative: a group missing from it is detached.",
			},
		},
	}
}

// alertingRuleGroupsURL builds the attachment endpoint for one rule.
func alertingRuleGroupsURL(client *APIClient, ruleID string) string {
	return fmt.Sprintf("%s/observability/alerting/rules/%s/groups", client.Endpoint, ruleID)
}

func resourceAlertingRuleGroupsWrite(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	ruleID := strconv.Itoa(d.Get("rule_id").(int))

	ids, _ := d.Get("endpoint_group_ids").([]interface{})
	// The endpoint replaces the attachments outright, so an empty list is a
	// meaningful request rather than something to omit: it detaches the rule
	// from every group.
	payload := map[string]interface{}{"endpointGroupIds": toIntSlice(ids)}

	if err := doJSON(ctx, client, http.MethodPut, alertingRuleGroupsURL(client, ruleID), payload, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to attach alert rule %s to its endpoint groups: %w", ruleID, err))
	}

	d.SetId(ruleID)
	return resourceAlertingRuleGroupsRead(ctx, d, meta)
}

func resourceAlertingRuleGroupsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	ruleID, err := strconv.Atoi(d.Id())
	if err != nil {
		return diag.FromErr(fmt.Errorf("the resource ID must be an alert rule identifier, got %q: %w", d.Id(), err))
	}

	var rule struct {
		ID               int   `json:"id"`
		EndpointGroupIDs []int `json:"endpointGroupIds"`
	}
	url := fmt.Sprintf("%s/observability/alerting/rules/%d", client.Endpoint, ruleID)
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &rule); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read alert rule %d: %w", ruleID, err))
	}

	if err := setFields(d, map[string]interface{}{
		"rule_id":            ruleID,
		"endpoint_group_ids": rule.EndpointGroupIDs,
	}); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceAlertingRuleGroupsDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	// Destroying the resource removes the attachments it created. The rule
	// itself is left alone: it may well be a built-in rule that Terraform has
	// no business deleting.
	payload := map[string]interface{}{"endpointGroupIds": []int{}}
	if err := doJSON(ctx, client, http.MethodPut, alertingRuleGroupsURL(client, d.Id()), payload, nil); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to detach alert rule %s from its endpoint groups: %w", d.Id(), err))
	}

	d.SetId("")
	return nil
}
