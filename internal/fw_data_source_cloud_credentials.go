package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*cloudCredentialsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*cloudCredentialsDataSource)(nil)
)

func newCloudCredentialsDataSource() datasource.DataSource { return &cloudCredentialsDataSource{} }

type cloudCredentialsDataSource struct{ client *APIClient }

type cloudCredentialsDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	CloudProvider types.String `tfsdk:"cloud_provider"`
}

func (d *cloudCredentialsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_credentials"
}

func (d *cloudCredentialsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the cloud credentials entry to look up in Portainer.",
			},
			"cloud_provider": schema.StringAttribute{
				Computed:    true,
				Description: "Cloud provider associated with the credentials (e.g. `civo`, `digitalocean`, `linode`, `gke`, `aws`, `azure`).",
			},
		},
	}
}

func (d *cloudCredentialsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *cloudCredentialsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m cloudCredentialsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := m.Name.ValueString()

	var credentials []struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Provider string `json:"provider"`
	}
	if err := doJSON(ctx, d.client, http.MethodGet, d.client.Endpoint+"/cloud/credentials", nil, &credentials); err != nil {
		resp.Diagnostics.AddError("Failed to list cloud credentials", fmt.Sprintf("failed to list cloud credentials: %s", err))
		return
	}
	for _, c := range credentials {
		if c.Name == name {
			m.ID = types.StringValue(strconv.Itoa(c.ID))
			m.CloudProvider = types.StringValue(c.Provider)
			resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
			return
		}
	}
	resp.Diagnostics.AddError("Cloud credentials not found", fmt.Sprintf("cloud credentials %s not found", name))
}
