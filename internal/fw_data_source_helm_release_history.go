package internal

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*helmReleaseHistoryDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*helmReleaseHistoryDataSource)(nil)
)

func newHelmReleaseHistoryDataSource() datasource.DataSource {
	return &helmReleaseHistoryDataSource{}
}

type helmReleaseHistoryDataSource struct{ client *APIClient }

type helmReleaseHistoryDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	EndpointID  types.Int64  `tfsdk:"endpoint_id"`
	ReleaseName types.String `tfsdk:"release_name"`
	Namespace   types.String `tfsdk:"namespace"`
	Revisions   types.List   `tfsdk:"revisions"`
}

type helmRevisionModel struct {
	Revision    types.Int64  `tfsdk:"revision"`
	Status      types.String `tfsdk:"status"`
	Chart       types.String `tfsdk:"chart"`
	AppVersion  types.String `tfsdk:"app_version"`
	Description types.String `tfsdk:"description"`
	Updated     types.String `tfsdk:"updated"`
	Name        types.String `tfsdk:"name"`
	Namespace   types.String `tfsdk:"namespace"`
}

var helmRevisionAttrTypes = map[string]attr.Type{
	"revision":    types.Int64Type,
	"status":      types.StringType,
	"chart":       types.StringType,
	"app_version": types.StringType,
	"description": types.StringType,
	"updated":     types.StringType,
	"name":        types.StringType,
	"namespace":   types.StringType,
}

func (d *helmReleaseHistoryDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_helm_release_history"
}

func (d *helmReleaseHistoryDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the data source, built from the environment and the release name.",
			},
			"endpoint_id": schema.Int64Attribute{
				Required:    true,
				Description: "Environment (Endpoint) identifier",
			},
			"release_name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the Helm release",
			},
			"namespace": schema.StringAttribute{
				Optional:    true,
				Description: "Kubernetes namespace of the release",
			},
			"revisions": schema.ListAttribute{
				Computed:    true,
				Description: "List of release revisions",
				ElementType: types.ObjectType{AttrTypes: helmRevisionAttrTypes},
			},
		},
	}
}

func (d *helmReleaseHistoryDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *helmReleaseHistoryDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m helmReleaseHistoryDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	endpointID := m.EndpointID.ValueInt64()
	releaseName := m.ReleaseName.ValueString()

	p := fmt.Sprintf("/endpoints/%d/kubernetes/helm/%s/history", endpointID, releaseName)
	if v := m.Namespace.ValueString(); v != "" {
		p += "?namespace=" + v
	}

	var releases []map[string]interface{}
	if err := doJSON(ctx, d.client, "GET", d.client.Endpoint+p, nil, &releases); err != nil {
		resp.Diagnostics.AddError("Failed to get Helm release history", fmt.Errorf("failed to get Helm release history: %w", err).Error())
		return
	}

	// Missing fields read as zero values, as SDKv2 stored them.
	revisions := make([]helmRevisionModel, 0, len(releases))
	for _, r := range releases {
		var rev struct {
			revision                                                   int64
			status, chart, appVersion, description, updated, name, nsp string
		}
		if v, ok := r["version"].(float64); ok {
			rev.revision = int64(v)
		}
		rev.name, _ = r["name"].(string)
		rev.nsp, _ = r["namespace"].(string)
		rev.appVersion, _ = r["appVersion"].(string)

		if info, ok := r["info"].(map[string]interface{}); ok {
			rev.status, _ = info["status"].(string)
			rev.description, _ = info["description"].(string)
			rev.updated, _ = info["last_deployed"].(string)
		}

		if chart, ok := r["chart"].(map[string]interface{}); ok {
			if metaMap, ok := chart["metadata"].(map[string]interface{}); ok {
				chartName, _ := metaMap["name"].(string)
				chartVersion, _ := metaMap["version"].(string)
				if chartName != "" && chartVersion != "" {
					rev.chart = chartName + "-" + chartVersion
				} else {
					rev.chart = chartName
				}
			}
		}

		revisions = append(revisions, helmRevisionModel{
			Revision:    types.Int64Value(rev.revision),
			Status:      types.StringValue(rev.status),
			Chart:       types.StringValue(rev.chart),
			AppVersion:  types.StringValue(rev.appVersion),
			Description: types.StringValue(rev.description),
			Updated:     types.StringValue(rev.updated),
			Name:        types.StringValue(rev.name),
			Namespace:   types.StringValue(rev.nsp),
		})
	}

	list, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: helmRevisionAttrTypes}, revisions)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	m.ID = types.StringValue(fmt.Sprintf("helm-history-%d-%s", endpointID, releaseName))
	m.Revisions = list
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
