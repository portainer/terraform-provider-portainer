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
	_ datasource.DataSource              = (*autoUpdatesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*autoUpdatesDataSource)(nil)
)

// autoUpdatesDataSource reports Portainer's own automatic update runs. It is
// the history rather than the configuration: what schedules the updates lives
// in Portainer's auto-patch settings.
func newAutoUpdatesDataSource() datasource.DataSource { return &autoUpdatesDataSource{} }

type autoUpdatesDataSource struct{ client *APIClient }

type autoUpdatesDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	AutoUpdates types.List   `tfsdk:"auto_updates"`
}

type autoUpdateRunModel struct {
	Version   types.String `tfsdk:"version"`
	Status    types.String `tfsdk:"status"`
	StartedAt types.Int64  `tfsdk:"started_at"`
	DoneAt    types.Int64  `tfsdk:"done_at"`
}

var autoUpdateRunAttrTypes = map[string]attr.Type{
	"version":    types.StringType,
	"status":     types.StringType,
	"started_at": types.Int64Type,
	"done_at":    types.Int64Type,
}

func (d *autoUpdatesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_auto_updates"
}

func (d *autoUpdatesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the data source, always `portainer-auto-updates`.",
			},
			"auto_updates": schema.ListAttribute{
				Computed:    true,
				Description: "The automatic update runs Portainer has recorded.",
				ElementType: types.ObjectType{AttrTypes: autoUpdateRunAttrTypes},
			},
		},
	}
}

func (d *autoUpdatesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *autoUpdatesDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	var response struct {
		AutoUpdates []struct {
			Version       string `json:"version"`
			Status        string `json:"status"`
			StartedAtUnix int64  `json:"startedAtUnix"`
			DoneAtUnix    int64  `json:"doneAtUnix"`
		} `json:"autoUpdates"`
	}
	if err := doJSON(ctx, d.client, http.MethodGet, d.client.Endpoint+"/auto_updates", nil, &response); err != nil {
		resp.Diagnostics.AddError("Failed to list the automatic updates", fmt.Errorf("failed to list the automatic updates: %w", err).Error())
		return
	}

	runs := make([]autoUpdateRunModel, 0, len(response.AutoUpdates))
	for _, u := range response.AutoUpdates {
		runs = append(runs, autoUpdateRunModel{
			Version:   types.StringValue(u.Version),
			Status:    types.StringValue(u.Status),
			StartedAt: types.Int64Value(u.StartedAtUnix),
			DoneAt:    types.Int64Value(u.DoneAtUnix),
		})
	}
	list, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: autoUpdateRunAttrTypes}, runs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state := autoUpdatesDataSourceModel{ID: types.StringValue("portainer-auto-updates"), AutoUpdates: list}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
