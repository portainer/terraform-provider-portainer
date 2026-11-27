package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*roleDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*roleDataSource)(nil)
)

func newRoleDataSource() datasource.DataSource { return &roleDataSource{} }

type roleDataSource struct{ client *APIClient }

type roleDataSourceModel struct {
	ID    types.String          `tfsdk:"id"`
	Name  types.String          `tfsdk:"name"`
	Roles []roleDataSourceEntry `tfsdk:"roles"`
}

type roleDataSourceEntry struct {
	ID          int64  `tfsdk:"id"`
	Name        string `tfsdk:"name"`
	Description string `tfsdk:"description"`
	Priority    int64  `tfsdk:"priority"`
}

func (d *roleDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (d *roleDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "Identifier of the matching role when `name` is set, a synthetic timestamp otherwise."},
			"name": schema.StringAttribute{
				Optional:    true,
				Description: "Filter by role name. If set, only the matching role is returned.",
			},
			"roles": schema.ListAttribute{
				Computed:    true,
				Description: "List of Portainer roles returned from the API, optionally filtered by name.",
				ElementType: types.ObjectType{AttrTypes: map[string]attr.Type{
					"id":          types.Int64Type,
					"name":        types.StringType,
					"description": types.StringType,
					"priority":    types.Int64Type,
				}},
			},
		},
	}
}

func (d *roleDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *roleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data roleDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var result []struct {
		ID          int64  `json:"Id"`
		Name        string `json:"Name"`
		Description string `json:"Description"`
		Priority    int64  `json:"Priority"`
	}
	if err := doJSON(ctx, d.client, http.MethodGet, d.client.Endpoint+"/roles", nil, &result); err != nil {
		resp.Diagnostics.AddError("Failed to read roles", fmt.Errorf("failed to list roles: %w", err).Error())
		return
	}

	// An empty name filters nothing, as SDKv2's GetOk treated it.
	nameFilter := data.Name.ValueString()
	nameFilterSet := nameFilter != ""

	data.Roles = []roleDataSourceEntry{}
	for _, r := range result {
		if nameFilterSet && r.Name != nameFilter {
			continue
		}
		data.Roles = append(data.Roles, roleDataSourceEntry{ID: r.ID, Name: r.Name, Description: r.Description, Priority: r.Priority})
	}

	if nameFilterSet && len(data.Roles) == 0 {
		resp.Diagnostics.AddError("Failed to read roles", fmt.Sprintf("role with name %q not found", nameFilter))
		return
	}

	if nameFilterSet && len(data.Roles) == 1 {
		data.ID = types.StringValue(strconv.FormatInt(data.Roles[0].ID, 10))
	} else {
		data.ID = types.StringValue(strconv.FormatInt(time.Now().Unix(), 10))
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
