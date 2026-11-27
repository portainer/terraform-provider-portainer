package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*alertingRuleTiersResource)(nil)
	_ resource.ResourceWithConfigure   = (*alertingRuleTiersResource)(nil)
	_ resource.ResourceWithImportState = (*alertingRuleTiersResource)(nil)
)

// alertingRuleTiersResource configures the multi-severity tiers of an
// editable built-in alert rule: the same metric firing at warning above one
// threshold and critical above another.
//
// Portainer exposes this through its own endpoint because it applies to rules
// Terraform did not create, which is why it is a separate resource rather than
// a block on portainer_alerting_rule.
func newAlertingRuleTiersResource() resource.Resource { return &alertingRuleTiersResource{} }

type alertingRuleTiersResource struct{ client *APIClient }

type alertingRuleTiersModel struct {
	ID                types.String `tfsdk:"id"`
	RuleID            types.Int64  `tfsdk:"rule_id"`
	Enabled           types.Bool   `tfsdk:"enabled"`
	Description       types.String `tfsdk:"description"`
	ConditionOperator types.String `tfsdk:"condition_operator"`
	Duration          types.Int64  `tfsdk:"duration"`
	Tier              types.List   `tfsdk:"tier"`
	UseTiers          types.Bool   `tfsdk:"use_tiers"`
}

type alertingRuleTierBlockModel struct {
	Severity  types.String  `tfsdk:"severity"`
	Threshold types.Float64 `tfsdk:"threshold"`
	Enabled   types.Bool    `tfsdk:"enabled"`
}

var alertingRuleTierBlockAttrTypes = map[string]attr.Type{
	"severity":  types.StringType,
	"threshold": types.Float64Type,
	"enabled":   types.BoolType,
}

func (r *alertingRuleTiersResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alerting_rule_tiers"
}

func (r *alertingRuleTiersResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"rule_id": schema.Int64Attribute{
				Required:      true,
				Description:   "Identifier of the alert rule to configure. The rule has to be an editable internal rule; Portainer rejects the call for others.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Whether the rule is enabled.",
			},
			"description": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Description stored on the rule. Leave unset to keep what the rule already has.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"condition_operator": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Validators:    []validator.String{stringvalidator.OneOf(">", "<", "=", ">=", "<=")},
				Description:   "Comparison the thresholds are evaluated with: `>`, `<`, `=`, `>=` or `<=`. Some built-in rules fix the operator and reject a change to it.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"duration": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "How long the condition has to hold before the rule fires, in the unit the rule itself uses.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"use_tiers": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether Portainer actually evaluates the rule per tier. A rule that reports `false` here is still using its single threshold.",
			},
		},
		Blocks: map[string]schema.Block{
			// A list rather than a set: the tiers are an ordered ladder of
			// severities, and reordering them is a change worth showing in a
			// plan.
			"tier": schema.ListNestedBlock{
				Description: "Severity tiers of the rule, in order. Each tier fires at its own threshold.",
				Validators:  []validator.List{listvalidator.IsRequired(), listvalidator.SizeAtLeast(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"severity": schema.StringAttribute{
							Required:    true,
							Validators:  []validator.String{stringvalidator.OneOf("critical", "warning", "info")},
							Description: "Severity this tier raises: `critical`, `warning` or `info`.",
						},
						"threshold": schema.Float64Attribute{
							Required:    true,
							Description: "Value the metric is compared against for this tier.",
						},
						"enabled": schema.BoolAttribute{
							Optional:    true,
							Computed:    true,
							Default:     booldefault.StaticBool(true),
							Description: "Whether this tier is evaluated.",
						},
					},
				},
			},
		},
	}
}

func (r *alertingRuleTiersResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *alertingRuleTiersResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan alertingRuleTiersModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, &plan, &resp.State, &resp.Diagnostics)
}

func (r *alertingRuleTiersResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan alertingRuleTiersModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, &plan, &resp.State, &resp.Diagnostics)
}

func (r *alertingRuleTiersResource) write(ctx context.Context, plan *alertingRuleTiersModel, state *tfsdk.State, diags *diag.Diagnostics) {
	ruleID := strconv.FormatInt(plan.RuleID.ValueInt64(), 10)

	var blocks []alertingRuleTierBlockModel
	diags.Append(plan.Tier.ElementsAs(ctx, &blocks, false)...)
	if diags.HasError() {
		return
	}
	tiers := make([]alertingRuleTier, 0, len(blocks))
	for _, b := range blocks {
		tiers = append(tiers, alertingRuleTier{Severity: b.Severity.ValueString(), Threshold: b.Threshold.ValueFloat64(), Enabled: b.Enabled.ValueBool()})
	}

	payload := map[string]interface{}{
		"enabled": plan.Enabled.ValueBool(),
		"tiers":   tiers,
	}
	// The three optional fields are omitted while they are still unknown.
	// Portainer keeps the rule's existing value for an omitted field, and a
	// built-in rule's description is not something an unrelated apply should
	// blank on the first write.
	if v := plan.Description; !v.IsNull() && !v.IsUnknown() && v.ValueString() != "" {
		payload["description"] = v.ValueString()
	}
	if v := plan.ConditionOperator; !v.IsNull() && !v.IsUnknown() && v.ValueString() != "" {
		payload["conditionOperator"] = v.ValueString()
	}
	if v := plan.Duration; !v.IsNull() && !v.IsUnknown() && v.ValueInt64() != 0 {
		payload["duration"] = int(v.ValueInt64())
	}

	url := fmt.Sprintf("%s/observability/alerting/rules/%s/tiers", r.client.Endpoint, ruleID)
	if err := doJSON(ctx, r.client, http.MethodPut, url, payload, nil); err != nil {
		diags.AddError("Failed to update alert rule tiers", fmt.Sprintf("failed to update the tiers of alert rule %s: %s", ruleID, err))
		return
	}
	plan.ID = types.StringValue(ruleID)

	planned := *plan
	found := r.readInto(ctx, plan, diags)
	if diags.HasError() {
		return
	}
	if !found {
		diags.AddError("Alert rule not found", fmt.Sprintf("alert rule %s disappeared right after its tiers were set", ruleID))
		return
	}
	// What the configuration set is what this apply results in; should
	// Portainer report something else, the next refresh shows it (Terraform
	// rejects an apply whose result differs from its plan).
	plan.Tier = planned.Tier
	plan.Enabled = planned.Enabled
	if !planned.Description.IsUnknown() {
		plan.Description = planned.Description
	}
	if !planned.ConditionOperator.IsUnknown() {
		plan.ConditionOperator = planned.ConditionOperator
	}
	if !planned.Duration.IsUnknown() {
		plan.Duration = planned.Duration
	}
	diags.Append(state.Set(ctx, plan)...)
}

// readInto reads the rule into m, reporting false when it is gone.
func (r *alertingRuleTiersResource) readInto(ctx context.Context, m *alertingRuleTiersModel, diags *diag.Diagnostics) bool {
	ruleID, err := strconv.Atoi(m.ID.ValueString())
	if err != nil {
		diags.AddError("Invalid resource ID", fmt.Sprintf("the resource ID must be an alert rule identifier, got %q: %s", m.ID.ValueString(), err))
		return false
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
	url := fmt.Sprintf("%s/observability/alerting/rules/%d", r.client.Endpoint, ruleID)
	if err := doJSON(ctx, r.client, http.MethodGet, url, nil, &rule); err != nil {
		if isAPINotFound(err) {
			return false
		}
		diags.AddError("Failed to read alert rule", fmt.Sprintf("failed to read alert rule %d: %s", ruleID, err))
		return false
	}

	elems := make([]attr.Value, 0, len(rule.Tiers))
	for _, tier := range rule.Tiers {
		elems = append(elems, types.ObjectValueMust(alertingRuleTierBlockAttrTypes, map[string]attr.Value{
			"severity":  types.StringValue(tier.Severity),
			"threshold": types.Float64Value(tier.Threshold),
			"enabled":   types.BoolValue(tier.Enabled),
		}))
	}

	// description, condition_operator and duration are Optional+Computed, so
	// reading the server's value back into an attribute the configuration left
	// out is what keeps a plan clean rather than what makes it churn.
	m.RuleID = types.Int64Value(int64(ruleID))
	m.Enabled = types.BoolValue(rule.Enabled)
	m.Tier = types.ListValueMust(types.ObjectType{AttrTypes: alertingRuleTierBlockAttrTypes}, elems)
	m.Description = types.StringValue(rule.Description)
	m.ConditionOperator = types.StringValue(rule.ConditionOperator)
	m.Duration = types.Int64Value(int64(rule.Duration))
	m.UseTiers = types.BoolValue(rule.UseTiers)
	return true
}

func (r *alertingRuleTiersResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state alertingRuleTiersModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	found := r.readInto(ctx, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Delete only forgets the resource. Portainer has no endpoint to clear a
// rule's tier configuration; destroying the resource stops managing it and
// leaves the rule as configured, rather than guessing at a payload that might
// disable it.
func (r *alertingRuleTiersResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

func (r *alertingRuleTiersResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
