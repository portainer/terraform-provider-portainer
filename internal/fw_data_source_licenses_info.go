package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*licensesInfoDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*licensesInfoDataSource)(nil)
)

func newLicensesInfoDataSource() datasource.DataSource { return &licensesInfoDataSource{} }

type licensesInfoDataSource struct{ client *APIClient }

type licensesInfoDataSourceModel struct {
	ID               types.String `tfsdk:"id"`
	Valid            types.Bool   `tfsdk:"valid"`
	Type             types.Int64  `tfsdk:"type"`
	Company          types.String `tfsdk:"company"`
	Nodes            types.Int64  `tfsdk:"nodes"`
	ExpiresAt        types.Int64  `tfsdk:"expires_at"`
	EnforcedAt       types.Int64  `tfsdk:"enforced_at"`
	OveruseStartedAt types.Int64  `tfsdk:"overuse_started_at"`
}

func (d *licensesInfoDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_licenses_info"
}

func (d *licensesInfoDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":                 schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"valid":              schema.BoolAttribute{Computed: true, Description: "Whether the licensing on this instance is currently valid."},
			"type":               schema.Int64Attribute{Computed: true, Description: "Portainer's numeric licence type."},
			"company":            schema.StringAttribute{Computed: true, Description: "Company the licence is issued to."},
			"nodes":              schema.Int64Attribute{Computed: true, Description: "Nodes the licence covers."},
			"expires_at":         schema.Int64Attribute{Computed: true, Description: "Unix timestamp the licence expires at."},
			"enforced_at":        schema.Int64Attribute{Computed: true, Description: "Unix timestamp enforcement begins at, zero when nothing is being enforced."},
			"overuse_started_at": schema.Int64Attribute{Computed: true, Description: "Unix timestamp the instance started exceeding its node count, zero when it is within it. A non-zero value here is the one worth alerting on."},
		},
	}
}

func (d *licensesInfoDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *licensesInfoDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	var info struct {
		Valid                   bool   `json:"valid"`
		Type                    int    `json:"type"`
		Company                 string `json:"company"`
		Nodes                   int    `json:"nodes"`
		ExpiresAt               int64  `json:"expiresAt"`
		EnforcedAt              int64  `json:"enforcedAt"`
		OveruseStartedTimestamp int64  `json:"overuseStartedTimestamp"`
	}
	if err := doJSON(ctx, d.client, http.MethodGet, d.client.Endpoint+"/licenses/info", nil, &info); err != nil {
		resp.Diagnostics.AddError("Failed to read the licence information", fmt.Sprintf("failed to read the licence information: %s", err))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &licensesInfoDataSourceModel{
		ID:               types.StringValue("portainer-licenses-info"),
		Valid:            types.BoolValue(info.Valid),
		Type:             types.Int64Value(int64(info.Type)),
		Company:          types.StringValue(info.Company),
		Nodes:            types.Int64Value(int64(info.Nodes)),
		ExpiresAt:        types.Int64Value(info.ExpiresAt),
		EnforcedAt:       types.Int64Value(info.EnforcedAt),
		OveruseStartedAt: types.Int64Value(info.OveruseStartedTimestamp),
	})...)
}
