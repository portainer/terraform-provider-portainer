package internal

import (
	"context"
	"fmt"
	"net/http"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*alertingRulesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*alertingRulesDataSource)(nil)
)

// alertingRulesDataSource lists the alert rules, predefined and custom. Its
// main use is finding the ID of a predefined rule by name, so that
// portainer_alerting_rule can adopt it without a hard-coded rule_id: IDs are
// assigned per Portainer instance.
func newAlertingRulesDataSource() datasource.DataSource { return &alertingRulesDataSource{} }

type alertingRulesDataSource struct{ client *APIClient }

type alertingRulesDataSourceModel struct {
	ID       types.String `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	Category types.String `tfsdk:"category"`
	Rules    types.List   `tfsdk:"rules"`
}

// alertingRuleObjectAttrTypes is an element of `rules`: the attributes of
// portainer_alerting_rule except `id` and `created`.
var alertingRuleObjectAttrTypes = map[string]attr.Type{
	"rule_id":                     types.Int64Type,
	"name":                        types.StringType,
	"description":                 types.StringType,
	"summary":                     types.StringType,
	"category":                    types.StringType,
	"enabled":                     types.BoolType,
	"severity":                    types.StringType,
	"metric_type":                 types.StringType,
	"condition_operator":          types.StringType,
	"operator_fixed":              types.BoolType,
	"threshold":                   types.Float64Type,
	"duration":                    types.Int64Type,
	"for_duration_minutes":        types.Int64Type,
	"use_duration_as_for":         types.BoolType,
	"alert_manager_id":            types.Int64Type,
	"endpoint_group_ids":          types.ListType{ElemType: types.Int64Type},
	"tag_ids":                     types.ListType{ElemType: types.Int64Type},
	"promql_expr":                 types.StringType,
	"promql_template":             types.StringType,
	"promql_scalar_template":      types.StringType,
	"annotation_template":         types.StringType,
	"use_tiers":                   types.BoolType,
	"independent_tiers":           types.BoolType,
	"tiers":                       types.ListType{ElemType: types.ObjectType{AttrTypes: alertingRuleTierAttrTypes}},
	"is_editable":                 types.BoolType,
	"is_internal":                 types.BoolType,
	"labels":                      types.MapType{ElemType: types.StringType},
	"supported_agent_version":     types.StringType,
	"supported_environment_types": types.StringType,
	"created_at":                  types.StringType,
	"created_by":                  types.StringType,
	"updated_at":                  types.StringType,
}

// alertingRuleObjectValue flattens a rule into an element of `rules`, the
// same values portainer_alerting_rule reads.
func alertingRuleObjectValue(rule AlertingRule) attr.Value {
	var m alertingRuleModel
	m.applyAlertingRule(rule)
	labels := m.Labels
	if labels.IsNull() {
		labels = stringMapValue(nil)
	}
	return types.ObjectValueMust(alertingRuleObjectAttrTypes, map[string]attr.Value{
		"rule_id":                     m.RuleID,
		"name":                        m.Name,
		"description":                 m.Description,
		"summary":                     m.Summary,
		"category":                    m.Category,
		"enabled":                     m.Enabled,
		"severity":                    m.Severity,
		"metric_type":                 m.MetricType,
		"condition_operator":          m.ConditionOperator,
		"operator_fixed":              m.OperatorFixed,
		"threshold":                   m.Threshold,
		"duration":                    m.Duration,
		"for_duration_minutes":        m.ForDurationMinutes,
		"use_duration_as_for":         m.UseDurationAsFor,
		"alert_manager_id":            m.AlertManagerID,
		"endpoint_group_ids":          m.EndpointGroupIDs,
		"tag_ids":                     m.TagIDs,
		"promql_expr":                 m.PromqlExpr,
		"promql_template":             m.PromqlTemplate,
		"promql_scalar_template":      m.PromqlScalarTemplate,
		"annotation_template":         m.AnnotationTemplate,
		"use_tiers":                   m.UseTiers,
		"independent_tiers":           m.IndependentTiers,
		"tiers":                       m.Tiers,
		"is_editable":                 m.IsEditable,
		"is_internal":                 m.IsInternal,
		"labels":                      labels,
		"supported_agent_version":     m.SupportedAgentVersion,
		"supported_environment_types": m.SupportedEnvironmentTypes,
		"created_at":                  m.CreatedAt,
		"created_by":                  m.CreatedBy,
		"updated_at":                  m.UpdatedAt,
	})
}

func (d *alertingRulesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alerting_rules"
}

func (d *alertingRulesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"name": schema.StringAttribute{
				Optional:    true,
				Description: "Only list the rules with exactly this name.",
			},
			"category": schema.StringAttribute{
				Optional:    true,
				Description: "Only list the rules of this category: `portainer`, `security`, `environment`, `backup` or `addons`.",
				Validators:  []validator.String{stringvalidator.OneOf("portainer", "security", "environment", "backup", "addons")},
			},
			"rules": schema.ListAttribute{
				Computed:    true,
				ElementType: types.ObjectType{AttrTypes: alertingRuleObjectAttrTypes},
				Description: "The matching alert rules, ordered by ID. Each element has the attributes of `portainer_alerting_rule` except `created`.",
			},
		},
	}
}

func (d *alertingRulesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *alertingRulesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data alertingRulesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	client := d.client

	var rules []AlertingRule
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/observability/alerting/rules", nil, &rules); err != nil {
		err = explainUnsupported(ctx, client, err, "portainer_alerting_rules", requiresBE)
		resp.Diagnostics.AddError("Failed to list alert rules", fmt.Sprintf("failed to list alert rules: %s", err))
		return
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].ID < rules[j].ID })

	name := data.Name.ValueString()
	category := data.Category.ValueString()
	out := make([]attr.Value, 0, len(rules))
	for _, rule := range rules {
		if (name != "" && rule.Name != name) || (category != "" && rule.Category != category) {
			continue
		}
		out = append(out, alertingRuleObjectValue(rule))
	}

	data.Rules = types.ListValueMust(types.ObjectType{AttrTypes: alertingRuleObjectAttrTypes}, out)
	data.ID = types.StringValue(fmt.Sprintf("portainer-alerting-rules-%s-%s", category, name))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
