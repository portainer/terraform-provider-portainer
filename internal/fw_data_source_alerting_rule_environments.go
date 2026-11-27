package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*alertingRuleEnvironmentsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*alertingRuleEnvironmentsDataSource)(nil)
)

// alertingRuleEnvironmentsDataSource reports where an alert rule actually
// landed. Attaching a rule to an endpoint group is not the same as the rule
// being evaluated there - an agent can be too old, an environment the wrong
// type - so the delivery status is what tells an operator whether the
// attachment did anything.
func newAlertingRuleEnvironmentsDataSource() datasource.DataSource {
	return &alertingRuleEnvironmentsDataSource{}
}

type alertingRuleEnvironmentsDataSource struct{ client *APIClient }

type alertingRuleEnvironmentsModel struct {
	ID           types.String `tfsdk:"id"`
	RuleID       types.Int64  `tfsdk:"rule_id"`
	Total        types.Int64  `tfsdk:"total"`
	Active       types.Int64  `tfsdk:"active"`
	Pending      types.Int64  `tfsdk:"pending"`
	Error        types.Int64  `tfsdk:"error"`
	Excluded     types.Int64  `tfsdk:"excluded"`
	NotSupported types.Int64  `tfsdk:"not_supported"`
	Groups       types.List   `tfsdk:"groups"`
}

var alertingRuleEnvironmentAttrTypes = map[string]attr.Type{
	"endpoint_id": types.Int64Type,
	"name":        types.StringType,
	"status":      types.StringType,
	"reason_code": types.StringType,
	"message":     types.StringType,
	"updated_at":  types.Int64Type,
}

var alertingRuleEnvironmentGroupAttrTypes = map[string]attr.Type{
	"endpoint_group_id":   types.Int64Type,
	"endpoint_group_name": types.StringType,
	"size":                types.Int64Type,
	"environments":        types.ListType{ElemType: types.ObjectType{AttrTypes: alertingRuleEnvironmentAttrTypes}},
}

func (d *alertingRuleEnvironmentsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alerting_rule_environments"
}

func (d *alertingRuleEnvironmentsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	count := func(desc string) schema.Int64Attribute {
		return schema.Int64Attribute{Computed: true, Description: desc}
	}
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"rule_id": schema.Int64Attribute{
				Required:    true,
				Description: "Identifier of the alert rule to report on.",
			},
			"total":         count("Number of environments the rule reaches, across every status below."),
			"active":        count("Number of environments where the rule is being evaluated."),
			"pending":       count("Number of environments where the rule has not been delivered yet."),
			"error":         count("Number of environments where delivering the rule failed. A non-zero value is the one worth alerting on."),
			"excluded":      count("Number of environments deliberately left out of the rule."),
			"not_supported": count("Number of environments that cannot run the rule, typically an agent too old or an environment of the wrong type."),
			"groups": schema.ListAttribute{
				Computed:    true,
				ElementType: types.ObjectType{AttrTypes: alertingRuleEnvironmentGroupAttrTypes},
				Description: "Delivery status broken down by endpoint group.",
			},
		},
	}
}

func (d *alertingRuleEnvironmentsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *alertingRuleEnvironmentsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data alertingRuleEnvironmentsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ruleID := data.RuleID.ValueInt64()

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

	url := fmt.Sprintf("%s/observability/alerting/rules/%d/environments", d.client.Endpoint, ruleID)
	if err := doJSON(ctx, d.client, http.MethodGet, url, nil, &response); err != nil {
		resp.Diagnostics.AddError("Failed to read alert rule environments", fmt.Sprintf("failed to read the environments of alert rule %d: %s", ruleID, err))
		return
	}

	envType := types.ObjectType{AttrTypes: alertingRuleEnvironmentAttrTypes}
	groups := make([]attr.Value, 0, len(response.Groups))
	for _, group := range response.Groups {
		environments := make([]attr.Value, 0, len(group.Environments))
		for _, environment := range group.Environments {
			environments = append(environments, types.ObjectValueMust(alertingRuleEnvironmentAttrTypes, map[string]attr.Value{
				"endpoint_id": types.Int64Value(int64(environment.EndpointID)),
				"name":        types.StringValue(environment.Name),
				"status":      types.StringValue(environment.Status),
				"reason_code": types.StringValue(environment.ReasonCode),
				"message":     types.StringValue(environment.Message),
				"updated_at":  types.Int64Value(environment.UpdatedAt),
			}))
		}
		groups = append(groups, types.ObjectValueMust(alertingRuleEnvironmentGroupAttrTypes, map[string]attr.Value{
			"endpoint_group_id":   types.Int64Value(int64(group.EndpointGroupID)),
			"endpoint_group_name": types.StringValue(group.EndpointGroupName),
			"size":                types.Int64Value(int64(group.Size)),
			"environments":        types.ListValueMust(envType, environments),
		}))
	}

	data.Total = types.Int64Value(int64(response.Total))
	data.Active = types.Int64Value(int64(response.Active))
	data.Pending = types.Int64Value(int64(response.Pending))
	data.Error = types.Int64Value(int64(response.Error))
	data.Excluded = types.Int64Value(int64(response.Excluded))
	data.NotSupported = types.Int64Value(int64(response.NotSupported))
	data.Groups = types.ListValueMust(types.ObjectType{AttrTypes: alertingRuleEnvironmentGroupAttrTypes}, groups)
	data.ID = types.StringValue(fmt.Sprintf("portainer-alerting-rule-environments-%d", ruleID))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
