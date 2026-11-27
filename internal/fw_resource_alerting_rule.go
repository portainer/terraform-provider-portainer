package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*alertingRuleResource)(nil)
	_ resource.ResourceWithConfigure   = (*alertingRuleResource)(nil)
	_ resource.ResourceWithImportState = (*alertingRuleResource)(nil)
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

func newAlertingRuleResource() resource.Resource { return &alertingRuleResource{} }

type alertingRuleResource struct{ client *APIClient }

type alertingRuleModel struct {
	ID                        types.String  `tfsdk:"id"`
	RuleID                    types.Int64   `tfsdk:"rule_id"`
	Name                      types.String  `tfsdk:"name"`
	Description               types.String  `tfsdk:"description"`
	Summary                   types.String  `tfsdk:"summary"`
	Category                  types.String  `tfsdk:"category"`
	Enabled                   types.Bool    `tfsdk:"enabled"`
	Severity                  types.String  `tfsdk:"severity"`
	MetricType                types.String  `tfsdk:"metric_type"`
	ConditionOperator         types.String  `tfsdk:"condition_operator"`
	Threshold                 types.Float64 `tfsdk:"threshold"`
	Duration                  types.Int64   `tfsdk:"duration"`
	ForDurationMinutes        types.Int64   `tfsdk:"for_duration_minutes"`
	AlertManagerID            types.Int64   `tfsdk:"alert_manager_id"`
	EndpointGroupIDs          types.List    `tfsdk:"endpoint_group_ids"`
	TagIDs                    types.List    `tfsdk:"tag_ids"`
	PromqlExpr                types.String  `tfsdk:"promql_expr"`
	SupportedEnvironmentTypes types.String  `tfsdk:"supported_environment_types"`
	Labels                    types.Map     `tfsdk:"labels"`
	Created                   types.Bool    `tfsdk:"created"`
	IsEditable                types.Bool    `tfsdk:"is_editable"`
	IsInternal                types.Bool    `tfsdk:"is_internal"`
	OperatorFixed             types.Bool    `tfsdk:"operator_fixed"`
	UseDurationAsFor          types.Bool    `tfsdk:"use_duration_as_for"`
	UseTiers                  types.Bool    `tfsdk:"use_tiers"`
	IndependentTiers          types.Bool    `tfsdk:"independent_tiers"`
	Tiers                     types.List    `tfsdk:"tiers"`
	PromqlTemplate            types.String  `tfsdk:"promql_template"`
	PromqlScalarTemplate      types.String  `tfsdk:"promql_scalar_template"`
	AnnotationTemplate        types.String  `tfsdk:"annotation_template"`
	SupportedAgentVersion     types.String  `tfsdk:"supported_agent_version"`
	CreatedAt                 types.String  `tfsdk:"created_at"`
	CreatedBy                 types.String  `tfsdk:"created_by"`
	UpdatedAt                 types.String  `tfsdk:"updated_at"`
}

func (r *alertingRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alerting_rule"
}

func (r *alertingRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	optString := func(desc string, validators ...validator.String) schema.StringAttribute {
		return schema.StringAttribute{
			Optional: true, Computed: true, Description: desc, Validators: validators,
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		}
	}
	optInt := func(desc string) schema.Int64Attribute {
		return schema.Int64Attribute{
			Optional: true, Computed: true, Description: desc,
			PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
		}
	}
	optIntList := func(desc string) schema.ListAttribute {
		return schema.ListAttribute{
			Optional: true, Computed: true, ElementType: types.Int64Type, Description: desc,
			PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
		}
	}
	compBool := func(desc string) schema.BoolAttribute {
		return schema.BoolAttribute{Computed: true, Description: desc}
	}
	compString := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{Computed: true, Description: desc}
	}

	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"rule_id": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "ID of the alert rule. Set it to adopt an existing rule (typically one of Portainer's predefined rules); leave it unset to create a custom rule, whose ID is then reported here.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
					int64planmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"name":        optString("Name of the alert rule. Required when creating a custom rule (`rule_id` unset)."),
			"description": optString("Description of the alert rule."),
			"summary":     optString("Summary of the alert rule."),
			"category": optString("Category the rule is grouped under: `portainer`, `security`, `environment`, `backup` or `addons` (`backup` and `addons` need Portainer BE 3.0 or later).",
				stringvalidator.OneOf("portainer", "security", "environment", "backup", "addons")),
			"enabled": schema.BoolAttribute{
				Required:    true,
				Description: "Whether the alert rule is enabled.",
			},
			"severity": optString("Severity level of the alert rule: `critical`, `warning` or `info`.",
				stringvalidator.OneOf("critical", "warning", "info")),
			"metric_type": optString("Unit of the metric the threshold is compared against: `percentage`, `bytes`, `raw` or `days`.",
				stringvalidator.OneOf("percentage", "bytes", "raw", "days")),
			"condition_operator": optString("Condition operator for threshold comparison: `>`, `<`, `=`, `>=` or `<=`. A rule with `operator_fixed` rejects a change.",
				stringvalidator.OneOf(">", "<", "=", ">=", "<=")),
			"threshold": schema.Float64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Threshold value for the alert rule.",
				PlanModifiers: []planmodifier.Float64{float64planmodifier.UseStateForUnknown()},
			},
			"duration":             optInt("Period within which the condition and threshold must hold for the rule to fire, in the unit the rule itself uses."),
			"for_duration_minutes": optInt("Minutes the condition has to keep holding before the alert fires (the `for` clause of the generated Prometheus rule)."),
			"alert_manager_id":     optInt("ID of the associated AlertManager settings."),
			"endpoint_group_ids":   optIntList("IDs of the endpoint groups the rule is restricted to; empty means every relevant environment. Do not also manage the same rule with `portainer_alerting_rule_groups`."),
			"tag_ids":              optIntList("IDs of the environment tags the rule is restricted to."),
			"promql_expr":          optString("PromQL expression the rule evaluates; this is what a custom rule measures."),
			"supported_environment_types": optString("Environment types the rule applies to: `docker`, `kubernetes`, `podman` or `all`.",
				stringvalidator.OneOf("docker", "kubernetes", "podman", "all")),
			"labels": schema.MapAttribute{
				Optional:      true,
				Computed:      true,
				ElementType:   types.StringType,
				Description:   "Labels associated with the alert rule.",
				PlanModifiers: []planmodifier.Map{mapplanmodifier.UseStateForUnknown()},
			},
			"created": schema.BoolAttribute{
				Computed:      true,
				Description:   "Whether Terraform created this rule. Only a created rule is deleted from Portainer on destroy; an adopted or imported rule is only removed from the state.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"is_editable":         compBool("Whether the alert rule can be edited."),
			"is_internal":         compBool("Whether the alert rule is one of Portainer's built-in rules."),
			"operator_fixed":      compBool("Whether the rule's definition fixes its condition operator."),
			"use_duration_as_for": compBool("Whether the rule uses `duration` as the `for` clause of its Prometheus rule."),
			"use_tiers":           compBool("Whether the rule is evaluated per severity tier rather than against its single threshold."),
			"independent_tiers":   compBool("Whether each tier measures its own series (Portainer BE 3.0 or later; always false before)."),
			"tiers": schema.ListAttribute{
				Computed:    true,
				ElementType: types.ObjectType{AttrTypes: alertingRuleTierAttrTypes},
				Description: "Severity tiers of the rule. Configure them with `portainer_alerting_rule_tiers`.",
			},
			"promql_template":         compString("PromQL template of a built-in rule, part of the rule's definition."),
			"promql_scalar_template":  compString("Scalar PromQL template of a built-in rule, part of the rule's definition."),
			"annotation_template":     compString("Template of the annotation the rule's alerts carry, part of the rule's definition."),
			"supported_agent_version": compString("Minimum agent version that supports this rule."),
			"created_at":              compString("Timestamp when the rule was created."),
			"created_by":              compString("User who created the rule."),
			"updated_at":              compString("Timestamp when the rule was last updated."),
		},
	}
}

func (r *alertingRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *alertingRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, config alertingRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	client := r.client

	// With rule_id the resource adopts an existing rule - most often one of
	// the rules Portainer predefines, which cannot be created - and applies
	// the configuration to it with the update call.
	if !plan.RuleID.IsNull() && !plan.RuleID.IsUnknown() && plan.RuleID.ValueInt64() != 0 {
		id := strconv.FormatInt(plan.RuleID.ValueInt64(), 10)
		// created stays false: an adopted rule outlives the resource.
		plan.Created = types.BoolValue(false)
		if err := alertingRuleUpdateCall(ctx, client, id, alertingRulePayload(ctx, plan.Enabled.ValueBool(), config, &resp.Diagnostics)); err != nil {
			resp.Diagnostics.AddError("Failed to update alert rule", err.Error())
			return
		}
		r.readInto(ctx, id, &plan, &resp.Diagnostics, func() {
			resp.Diagnostics.AddError("Alert rule not found", fmt.Sprintf("alert rule %s disappeared right after it was updated", id))
		})
		if resp.Diagnostics.HasError() {
			return
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}

	name := ""
	if !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		name = plan.Name.ValueString()
	}
	if name == "" {
		resp.Diagnostics.AddError("Missing alert rule name", "name is required to create a custom alert rule; set rule_id instead to adopt an existing rule")
		return
	}

	payload := alertingRulePayload(ctx, plan.Enabled.ValueBool(), config, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var created AlertingRule
	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/observability/alerting/rules", map[string]interface{}{"AlertingRule": payload}, &created); err != nil {
		resp.Diagnostics.AddError("Failed to create alert rule", fmt.Sprintf("failed to create alert rule %q: %s", name, err))
		return
	}

	// The spec types the response as a bare object. Should it not carry the
	// new rule's ID, the rule is looked up by name instead.
	if created.ID == 0 {
		id, err := findAlertingRuleByName(ctx, client, name)
		if err != nil {
			// The rule exists but its ID is unknown, so it cannot go into
			// state; say so, or the next apply silently creates a second one.
			resp.Diagnostics.AddError("Failed to determine the alert rule ID", fmt.Sprintf("alert rule %q was created, but its ID could not be determined (%v); import it with `terraform import` using its ID from the Portainer UI before applying again", name, err))
			return
		}
		created.ID = id
	}

	id := strconv.Itoa(created.ID)
	plan.Created = types.BoolValue(true)
	r.readInto(ctx, id, &plan, &resp.Diagnostics, func() {
		resp.Diagnostics.AddError("Alert rule not found", fmt.Sprintf("alert rule %s disappeared right after it was created", id))
	})
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
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

// readInto reads rule id into m; notFound runs on a 404.
func (r *alertingRuleResource) readInto(ctx context.Context, id string, m *alertingRuleModel, diags *diag.Diagnostics, notFound func()) {
	var rule AlertingRule
	if err := doJSON(ctx, r.client, http.MethodGet, fmt.Sprintf("%s/observability/alerting/rules/%s", r.client.Endpoint, id), nil, &rule); err != nil {
		if isAPINotFound(err) {
			notFound()
			return
		}
		diags.AddError("Failed to read alert rule", fmt.Sprintf("failed to read alert rule %s: %s", id, err))
		return
	}
	m.ID = types.StringValue(strconv.Itoa(rule.ID))
	// created is not part of the rule: it stays what Create set, and false
	// after an import or an adoption.
	if m.Created.IsNull() || m.Created.IsUnknown() {
		m.Created = types.BoolValue(false)
	}
	m.applyAlertingRule(rule)
}

func (m *alertingRuleModel) applyAlertingRule(rule AlertingRule) {
	m.RuleID = types.Int64Value(int64(rule.ID))
	m.Name = types.StringValue(rule.Name)
	m.Description = types.StringValue(rule.Description)
	m.Summary = types.StringValue(rule.Summary)
	m.Category = types.StringValue(rule.Category)
	m.Enabled = types.BoolValue(rule.Enabled)
	m.Severity = types.StringValue(rule.Severity)
	m.MetricType = types.StringValue(rule.MetricType)
	m.ConditionOperator = types.StringValue(rule.ConditionOperator)
	m.OperatorFixed = types.BoolValue(rule.OperatorFixed)
	m.Threshold = types.Float64Value(rule.Threshold)
	m.Duration = types.Int64Value(int64(rule.Duration))
	m.ForDurationMinutes = types.Int64Value(int64(rule.ForDurationMinutes))
	m.UseDurationAsFor = types.BoolValue(rule.UseDurationAsFor)
	m.AlertManagerID = types.Int64Value(int64(rule.AlertManagerID))
	m.EndpointGroupIDs = int64ListValue(rule.EndpointGroupIDs)
	m.TagIDs = int64ListValue(rule.TagIDs)
	m.PromqlExpr = types.StringValue(rule.PromqlExpr)
	m.PromqlTemplate = types.StringValue(rule.PromqlTemplate)
	m.PromqlScalarTemplate = types.StringValue(rule.PromqlScalarTemplate)
	m.AnnotationTemplate = types.StringValue(rule.AnnotationTemplate)
	m.UseTiers = types.BoolValue(rule.UseTiers)
	m.IndependentTiers = types.BoolValue(rule.IndependentTiers)
	m.Tiers = alertingRuleTiersValue(rule.Tiers)
	m.IsEditable = types.BoolValue(rule.IsEditable)
	m.IsInternal = types.BoolValue(rule.IsInternal)
	m.Labels = alertingLabelsValue(rule.Labels, m.Labels)
	m.SupportedAgentVersion = types.StringValue(rule.SupportedAgentVersion)
	m.SupportedEnvironmentTypes = types.StringValue(rule.SupportedEnvironmentTypes)
	m.CreatedAt = types.StringValue(rule.CreatedAt)
	m.CreatedBy = types.StringValue(rule.CreatedBy)
	m.UpdatedAt = types.StringValue(rule.UpdatedAt)
}

func (r *alertingRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state alertingRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	removed := false
	r.readInto(ctx, state.ID.ValueString(), &state, &resp.Diagnostics, func() { removed = true })
	if resp.Diagnostics.HasError() {
		return
	}
	if removed {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *alertingRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state, config alertingRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	payload := alertingRulePayload(ctx, plan.Enabled.ValueBool(), config, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := alertingRuleUpdateCall(ctx, r.client, id, payload); err != nil {
		resp.Diagnostics.AddError("Failed to update alert rule", err.Error())
		return
	}
	plan.Created = state.Created
	r.readInto(ctx, id, &plan, &resp.Diagnostics, func() {
		resp.Diagnostics.AddError("Alert rule not found", fmt.Sprintf("alert rule %s disappeared right after it was updated", id))
	})
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// alertingRuleUpdateCall lays payload over the rule as Portainer returns it
// and PUTs the result back.
func alertingRuleUpdateCall(ctx context.Context, client *APIClient, id string, payload map[string]interface{}) error {
	url := fmt.Sprintf("%s/observability/alerting/rules/%s", client.Endpoint, id)

	// Whether the PUT merges or replaces the rule is not documented. The rule
	// is read first and sent back whole with the configuration laid over it,
	// so a predefined rule keeps its PromQL, templates and tiers either way.
	rule := map[string]interface{}{}
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &rule); err != nil {
		return fmt.Errorf("failed to read alert rule %s before updating it: %w", id, err)
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
	for k, v := range payload {
		rule[k] = v
	}
	if n, err := strconv.Atoi(id); err == nil {
		rule["id"] = n
	}

	if err := doJSON(ctx, client, http.MethodPut, url, map[string]interface{}{"AlertingRule": rule}, nil); err != nil {
		return fmt.Errorf("failed to update alert rule %s: %w", id, err)
	}
	return nil
}

func (r *alertingRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state alertingRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// A rule Terraform did not create - a predefined rule it adopted, or one
	// it imported - outlives the resource: destroying it only stops managing
	// it.
	if !state.Created.ValueBool() {
		return
	}
	id := state.ID.ValueString()
	if err := doJSON(ctx, r.client, http.MethodDelete, fmt.Sprintf("%s/observability/alerting/rules/%s", r.client.Endpoint, id), nil, nil); err != nil && !isAPINotFound(err) {
		resp.Diagnostics.AddError("Failed to delete alert rule", fmt.Sprintf("failed to delete alert rule %s: %s", id, err))
	}
}

func (r *alertingRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// alertingRulePayload returns enabled plus the attributes the configuration
// sets: the whole body of a create, and what an update lays over the rule as
// Portainer returns it. An attribute the configuration never mentions is
// therefore never handed to Portainer as a zero value.
func alertingRulePayload(ctx context.Context, enabled bool, config alertingRuleModel, diags *diag.Diagnostics) map[string]interface{} {
	rule := map[string]interface{}{"enabled": enabled}
	set := func(v attr.Value) bool { return !v.IsNull() && !v.IsUnknown() }
	strs := []struct {
		key string
		v   types.String
	}{
		{"name", config.Name},
		{"description", config.Description},
		{"summary", config.Summary},
		{"category", config.Category},
		{"severity", config.Severity},
		{"metricType", config.MetricType},
		{"conditionOperator", config.ConditionOperator},
		{"promqlExpr", config.PromqlExpr},
		{"supportedEnvironmentTypes", config.SupportedEnvironmentTypes},
	}
	for _, s := range strs {
		if set(s.v) {
			rule[s.key] = s.v.ValueString()
		}
	}
	if set(config.Threshold) {
		rule["threshold"] = config.Threshold.ValueFloat64()
	}
	ints := []struct {
		key string
		v   types.Int64
	}{
		{"duration", config.Duration},
		{"forDurationMinutes", config.ForDurationMinutes},
		{"alertManagerID", config.AlertManagerID},
	}
	for _, i := range ints {
		if set(i.v) {
			rule[i.key] = int(i.v.ValueInt64())
		}
	}
	if set(config.EndpointGroupIDs) {
		rule["endpointGroupIds"] = int64ListToInts(ctx, config.EndpointGroupIDs, diags)
	}
	if set(config.TagIDs) {
		rule["tagIds"] = int64ListToInts(ctx, config.TagIDs, diags)
	}
	if set(config.Labels) {
		rule["labels"] = stringMapToGo(ctx, config.Labels, diags)
	}
	return rule
}
