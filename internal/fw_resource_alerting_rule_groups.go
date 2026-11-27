package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*alertingRuleGroupsResource)(nil)
	_ resource.ResourceWithConfigure   = (*alertingRuleGroupsResource)(nil)
	_ resource.ResourceWithImportState = (*alertingRuleGroupsResource)(nil)
)

// alertingRuleGroupsResource attaches an alert rule to endpoint groups.
//
// It is a resource of its own rather than a field on portainer_alerting_rule
// because most rules worth attaching are Portainer's own built-in ones: they
// are not created by Terraform and cannot be, but which environments they
// cover is exactly what an operator wants under version control.
func newAlertingRuleGroupsResource() resource.Resource { return &alertingRuleGroupsResource{} }

type alertingRuleGroupsResource struct{ client *APIClient }

type alertingRuleGroupsModel struct {
	ID               types.String `tfsdk:"id"`
	RuleID           types.Int64  `tfsdk:"rule_id"`
	EndpointGroupIDs types.List   `tfsdk:"endpoint_group_ids"`
}

func (r *alertingRuleGroupsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alerting_rule_groups"
}

func (r *alertingRuleGroupsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"rule_id": schema.Int64Attribute{
				Required:      true,
				Description:   "Identifier of the alert rule whose endpoint group attachments this resource owns. Built-in rules can be attached the same way rules created by `portainer_alerting_rule` can.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"endpoint_group_ids": schema.ListAttribute{
				Required:    true,
				ElementType: types.Int64Type,
				Description: "Identifiers of the endpoint groups the rule applies to. The list is authoritative: a group missing from it is detached.",
			},
		},
	}
}

func (r *alertingRuleGroupsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

// alertingRuleGroupsURL builds the attachment endpoint for one rule.
func alertingRuleGroupsURL(client *APIClient, ruleID string) string {
	return fmt.Sprintf("%s/observability/alerting/rules/%s/groups", client.Endpoint, ruleID)
}

func (r *alertingRuleGroupsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan alertingRuleGroupsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, &plan, &resp.State, &resp.Diagnostics)
}

func (r *alertingRuleGroupsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan alertingRuleGroupsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, &plan, &resp.State, &resp.Diagnostics)
}

func (r *alertingRuleGroupsResource) write(ctx context.Context, plan *alertingRuleGroupsModel, state *tfsdk.State, diags *diag.Diagnostics) {
	ruleID := strconv.FormatInt(plan.RuleID.ValueInt64(), 10)
	// The endpoint replaces the attachments outright, so an empty list is a
	// meaningful request rather than something to omit: it detaches the rule
	// from every group.
	ids := int64ListToInts(ctx, plan.EndpointGroupIDs, diags)
	if ids == nil {
		ids = []int{}
	}
	payload := map[string]interface{}{"endpointGroupIds": ids}
	if err := doJSON(ctx, r.client, http.MethodPut, alertingRuleGroupsURL(r.client, ruleID), payload, nil); err != nil {
		diags.AddError("Failed to attach alert rule", fmt.Sprintf("failed to attach alert rule %s to its endpoint groups: %s", ruleID, err))
		return
	}
	plan.ID = types.StringValue(ruleID)

	// The attachments are read back from the rule. Should Portainer report
	// another list than the one just sent, the state keeps the configured
	// list (Terraform rejects an apply whose result differs from its plan)
	// and the next refresh shows the difference.
	current, found, err := r.readGroups(ctx, plan.RuleID.ValueInt64())
	if err != nil {
		diags.AddError("Failed to read alert rule", err.Error())
		return
	}
	if !found {
		diags.AddError("Alert rule not found", fmt.Sprintf("alert rule %s disappeared right after its endpoint groups were set", ruleID))
		return
	}
	if alertingSameIntSlices(current, ids) {
		plan.EndpointGroupIDs = int64ListValue(current)
	}
	diags.Append(state.Set(ctx, plan)...)
}

// readGroups returns the endpoint groups rule ruleID is attached to.
func (r *alertingRuleGroupsResource) readGroups(ctx context.Context, ruleID int64) ([]int, bool, error) {
	var rule struct {
		ID               int   `json:"id"`
		EndpointGroupIDs []int `json:"endpointGroupIds"`
	}
	url := fmt.Sprintf("%s/observability/alerting/rules/%d", r.client.Endpoint, ruleID)
	if err := doJSON(ctx, r.client, http.MethodGet, url, nil, &rule); err != nil {
		if isAPINotFound(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("failed to read alert rule %d: %w", ruleID, err)
	}
	return rule.EndpointGroupIDs, true, nil
}

func alertingSameIntSlices(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (r *alertingRuleGroupsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state alertingRuleGroupsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ruleID, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid resource ID", fmt.Sprintf("the resource ID must be an alert rule identifier, got %q: %s", state.ID.ValueString(), err))
		return
	}

	groups, found, err := r.readGroups(ctx, int64(ruleID))
	if err != nil {
		resp.Diagnostics.AddError("Failed to read alert rule", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	state.RuleID = types.Int64Value(int64(ruleID))
	state.EndpointGroupIDs = int64ListValue(groups)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *alertingRuleGroupsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state alertingRuleGroupsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Destroying the resource removes the attachments it created. The rule
	// itself is left alone: it may well be a built-in rule that Terraform has
	// no business deleting.
	payload := map[string]interface{}{"endpointGroupIds": []int{}}
	if err := doJSON(ctx, r.client, http.MethodPut, alertingRuleGroupsURL(r.client, state.ID.ValueString()), payload, nil); err != nil && !isAPINotFound(err) {
		resp.Diagnostics.AddError("Failed to detach alert rule", fmt.Sprintf("failed to detach alert rule %s from its endpoint groups: %s", state.ID.ValueString(), err))
	}
}

func (r *alertingRuleGroupsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
