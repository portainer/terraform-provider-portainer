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
	_ datasource.DataSource              = (*endpointsSummaryDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*endpointsSummaryDataSource)(nil)
)

func newEndpointsSummaryDataSource() datasource.DataSource { return &endpointsSummaryDataSource{} }

type endpointsSummaryDataSource struct{ client *APIClient }

type endpointsSummaryModel struct {
	ID         types.String                 `tfsdk:"id"`
	Total      types.Int64                  `tfsdk:"total"`
	Up         types.Int64                  `tfsdk:"up"`
	Down       types.Int64                  `tfsdk:"down"`
	Heartbeat  types.Int64                  `tfsdk:"heartbeat"`
	Outdated   types.Int64                  `tfsdk:"outdated"`
	Unassigned types.Int64                  `tfsdk:"unassigned"`
	Docker     types.Int64                  `tfsdk:"docker"`
	Kubernetes types.Int64                  `tfsdk:"kubernetes"`
	Azure      types.Int64                  `tfsdk:"azure"`
	Podman     types.Int64                  `tfsdk:"podman"`
	ByGroup    []endpointsSummaryGroupModel `tfsdk:"by_group"`
}

type endpointsSummaryGroupModel struct {
	GroupID   types.Int64  `tfsdk:"group_id"`
	GroupName types.String `tfsdk:"group_name"`
	Count     types.Int64  `tfsdk:"count"`
}

func (d *endpointsSummaryDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_endpoints_summary"
}

func (d *endpointsSummaryDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	count := func(description string) schema.Attribute {
		return schema.Int64Attribute{Computed: true, Description: description}
	}
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":         schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"total":      count("Total number of environments."),
			"up":         count("Environments Portainer can currently reach."),
			"down":       count("Environments Portainer cannot reach."),
			"heartbeat":  count("Edge environments whose agent is checking in."),
			"outdated":   count("Environments running an agent older than the Portainer server."),
			"unassigned": count("Environments still in the Unassigned group."),
			"docker":     count("Number of Docker environments."),
			"kubernetes": count("Number of Kubernetes environments."),
			"azure":      count("Number of Azure ACI environments."),
			"podman":     count("Number of Podman environments."),
			// Protocol 5 has no nested attributes: a computed list of objects,
			// as SDKv2 declared it.
			"by_group": schema.ListAttribute{
				Computed:    true,
				Description: "Environment count per environment group.",
				ElementType: types.ObjectType{AttrTypes: map[string]attr.Type{
					"group_id":   types.Int64Type,
					"group_name": types.StringType,
					"count":      types.Int64Type,
				}},
			},
		},
	}
}

func (d *endpointsSummaryDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *endpointsSummaryDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	var summary struct {
		Total      int `json:"total"`
		Up         int `json:"up"`
		Down       int `json:"down"`
		Outdated   int `json:"outdated"`
		Unassigned int `json:"unassigned"`
		ByHealth   struct {
			Up        int `json:"up"`
			Down      int `json:"down"`
			Heartbeat int `json:"heartbeat"`
			Outdated  int `json:"outdated"`
		} `json:"byHealth"`
		ByPlatformType struct {
			Docker     int `json:"docker"`
			Kubernetes int `json:"kubernetes"`
			Azure      int `json:"azure"`
			Podman     int `json:"podman"`
		} `json:"byPlatformType"`
		ByGroup []struct {
			GroupID   int    `json:"groupId"`
			GroupName string `json:"groupName"`
			Count     int    `json:"count"`
		} `json:"byGroup"`
	}
	if err := doJSON(ctx, d.client, http.MethodGet, d.client.Endpoint+"/endpoints/summary", nil, &summary); err != nil {
		err = explainUnsupported(ctx, d.client, err, "portainer_endpoints_summary", requires241)
		resp.Diagnostics.AddError("Failed to read the environment summary", fmt.Errorf("failed to read the environment summary: %w", err).Error())
		return
	}

	groups := make([]endpointsSummaryGroupModel, len(summary.ByGroup))
	for i, g := range summary.ByGroup {
		groups[i] = endpointsSummaryGroupModel{
			GroupID:   types.Int64Value(int64(g.GroupID)),
			GroupName: types.StringValue(g.GroupName),
			Count:     types.Int64Value(int64(g.Count)),
		}
	}

	// The health counters live under byHealth; the top-level up/down/outdated
	// fields repeat them, so the nested ones win when they are populated.
	up, down, outdated := summary.Up, summary.Down, summary.Outdated
	if summary.ByHealth.Up != 0 || summary.ByHealth.Down != 0 || summary.ByHealth.Outdated != 0 {
		up, down, outdated = summary.ByHealth.Up, summary.ByHealth.Down, summary.ByHealth.Outdated
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &endpointsSummaryModel{
		ID:         types.StringValue("portainer-endpoints-summary"),
		Total:      types.Int64Value(int64(summary.Total)),
		Up:         types.Int64Value(int64(up)),
		Down:       types.Int64Value(int64(down)),
		Heartbeat:  types.Int64Value(int64(summary.ByHealth.Heartbeat)),
		Outdated:   types.Int64Value(int64(outdated)),
		Unassigned: types.Int64Value(int64(summary.Unassigned)),
		Docker:     types.Int64Value(int64(summary.ByPlatformType.Docker)),
		Kubernetes: types.Int64Value(int64(summary.ByPlatformType.Kubernetes)),
		Azure:      types.Int64Value(int64(summary.ByPlatformType.Azure)),
		Podman:     types.Int64Value(int64(summary.ByPlatformType.Podman)),
		ByGroup:    groups,
	})...)
}
