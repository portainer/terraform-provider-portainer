package internal

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSourceWithConfigure = (*registryDataSource)(nil)

func newRegistryDataSource() datasource.DataSource { return &registryDataSource{} }

type registryDataSource struct{ client *APIClient }

type registryDataSourceModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
	URL  types.String `tfsdk:"url"`
	Type types.Int64  `tfsdk:"type"`
}

func (d *registryDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_registry"
}

func (d *registryDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":   schema.StringAttribute{Computed: true, Description: "Identifier of the registry."},
			"name": schema.StringAttribute{Required: true, Description: "Name of the Portainer registry to look up. The data source will fail if no matching registry is found."},
			"url":  schema.StringAttribute{Computed: true, Description: "URL of the container registry as registered in Portainer."},
			"type": schema.Int64Attribute{Computed: true, Description: "Type of the Portainer registry (e.g. Quay, Azure, custom, GitLab, ProGet, DockerHub, ECR, GitHub)."},
		},
	}
}

func (d *registryDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *registryDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data registryDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := data.Name.ValueString()
	registryList, err := listRegistries(ctx, d.client)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read registry", err.Error())
		return
	}
	for _, r := range registryList {
		if r.Name == name {
			data.ID = types.StringValue(strconv.FormatInt(r.ID, 10))
			data.URL = types.StringValue(r.URL)
			data.Type = types.Int64Value(r.Type)
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}
	resp.Diagnostics.AddError("Failed to read registry", fmt.Sprintf("registry %s not found", name))
}
