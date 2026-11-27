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

var _ datasource.DataSourceWithConfigure = (*edgeWaitingRoomDataSource)(nil)

func newEdgeWaitingRoomDataSource() datasource.DataSource { return &edgeWaitingRoomDataSource{} }

// edgeWaitingRoomDataSource lists the edge environments that have called home
// but have not been trusted yet - what the Portainer UI calls the waiting
// room. It is the companion to portainer_endpoint_trust: the environments are
// created by the agents, so their identifiers are not known to a configuration
// until they are looked up.
type edgeWaitingRoomDataSource struct{ edgeDataSourceBase }

type edgeWaitingRoomModel struct {
	ID           types.String `tfsdk:"id"`
	GroupID      types.Int64  `tfsdk:"group_id"`
	EndpointIDs  types.List   `tfsdk:"endpoint_ids"`
	Environments types.List   `tfsdk:"environments"`
}

var edgeWaitingRoomEnvAttrTypes = map[string]attr.Type{
	"id": types.Int64Type, "name": types.StringType, "type": types.Int64Type, "edge_id": types.StringType,
	"group_id": types.Int64Type, "last_check_in": types.Int64Type,
}

func (d *edgeWaitingRoomDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_edge_waiting_room"
}

func (d *edgeWaitingRoomDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "Always `portainer-edge-waiting-room`."},
			"group_id": schema.Int64Attribute{
				Optional:    true,
				Description: "Only list environments in this endpoint group. Leave unset to list the whole waiting room.",
			},
			"endpoint_ids": schema.ListAttribute{
				Computed:    true,
				ElementType: types.Int64Type,
				Description: "Identifiers of the environments awaiting trust, which is what `portainer_endpoint_trust` takes.",
			},
			"environments": schema.ListAttribute{
				Computed:    true,
				ElementType: types.ObjectType{AttrTypes: edgeWaitingRoomEnvAttrTypes},
				Description: "The environments awaiting trust.",
			},
		},
	}
}

func (d *edgeWaitingRoomDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m edgeWaitingRoomModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := d.client.Endpoint + "/endpoints?edgeDeviceUntrusted=true"
	if v := m.GroupID.ValueInt64(); v != 0 {
		url += fmt.Sprintf("&groupIds=%d", v)
	}

	var endpoints []struct {
		ID            int    `json:"Id"`
		Name          string `json:"Name"`
		Type          int    `json:"Type"`
		EdgeID        string `json:"EdgeID"`
		GroupID       int    `json:"GroupId"`
		LastCheckInAt int64  `json:"LastCheckInDate"`
	}
	if err := doJSON(ctx, d.client, http.MethodGet, url, nil, &endpoints); err != nil {
		resp.Diagnostics.AddError("Failed to list the waiting room", fmt.Sprintf("failed to list the edge environments awaiting trust: %s", err))
		return
	}

	ids := make([]int, 0, len(endpoints))
	environments := make([]map[string]attr.Value, 0, len(endpoints))
	for _, e := range endpoints {
		ids = append(ids, e.ID)
		environments = append(environments, map[string]attr.Value{
			"id": types.Int64Value(int64(e.ID)), "name": types.StringValue(e.Name), "type": types.Int64Value(int64(e.Type)),
			"edge_id": types.StringValue(e.EdgeID), "group_id": types.Int64Value(int64(e.GroupID)),
			"last_check_in": types.Int64Value(e.LastCheckInAt),
		})
	}
	m.ID = types.StringValue("portainer-edge-waiting-room")
	m.EndpointIDs = int64ListValue(ids)
	m.Environments = edgeObjectList(edgeWaitingRoomEnvAttrTypes, environments, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
