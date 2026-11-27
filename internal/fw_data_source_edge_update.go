package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// --- portainer_edge_update_schedule_info ---

var _ datasource.DataSourceWithConfigure = (*edgeUpdateScheduleInfoDataSource)(nil)

func newEdgeUpdateScheduleInfoDataSource() datasource.DataSource {
	return &edgeUpdateScheduleInfoDataSource{}
}

// edgeUpdateScheduleInfoDataSource folds the two argument-less endpoints that
// describe what an edge update schedule could do: the fleet's current state
// and the agent versions on offer.
type edgeUpdateScheduleInfoDataSource struct{ edgeDataSourceBase }

type edgeUpdateScheduleInfoModel struct {
	ID                  types.String `tfsdk:"id"`
	AgentVersions       types.List   `tfsdk:"agent_versions"`
	MinimumAgentVersion types.String `tfsdk:"minimum_agent_version"`
	UpToDateCount       types.Int64  `tfsdk:"up_to_date_count"`
	OutdatedCount       types.Int64  `tfsdk:"outdated_count"`
	HasLocalTimeZone    types.Bool   `tfsdk:"has_local_time_zone"`
	HasNoLocalTimeZone  types.Bool   `tfsdk:"has_no_local_time_zone"`
}

func (d *edgeUpdateScheduleInfoDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_edge_update_schedule_info"
}

func (d *edgeUpdateScheduleInfoDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "Always `portainer-edge-update-schedule-info`."},
			"agent_versions": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Agent versions an update schedule can move environments to, sorted.",
			},
			"minimum_agent_version": schema.StringAttribute{
				Computed:    true,
				Description: "Lowest agent version that can take part in an update schedule.",
			},
			"up_to_date_count": schema.Int64Attribute{
				Computed:    true,
				Description: "Environments already on the newest agent.",
			},
			"outdated_count": schema.Int64Attribute{
				Computed:    true,
				Description: "Environments on an older agent. This is the number an update schedule would act on.",
			},
			"has_local_time_zone": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether any environment reports a local timezone, which is what lets a schedule run in local time.",
			},
			"has_no_local_time_zone": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether any environment reports no local timezone. Those environments cannot be scheduled in local time.",
			},
		},
	}
}

func (d *edgeUpdateScheduleInfoDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	var info struct {
		MinAgentVersion    string `json:"MinAgentVersion"`
		UpToDateCount      int    `json:"UpToDateCount"`
		OutdatedCount      int    `json:"OutdatedCount"`
		HasLocalTimeZone   bool   `json:"HasLocalTimeZone"`
		HasNoLocalTimeZone bool   `json:"HasNoLocalTimeZone"`
	}
	if err := doJSON(ctx, d.client, http.MethodGet, d.client.Endpoint+"/edge_update_schedules/info", nil, &info); err != nil {
		resp.Diagnostics.AddError("Failed to read the edge update schedule information", fmt.Sprintf("failed to read the edge update schedule information: %s", err))
		return
	}
	var versions []string
	if err := doJSON(ctx, d.client, http.MethodGet, d.client.Endpoint+"/edge_update_schedules/agent_versions", nil, &versions); err != nil {
		resp.Diagnostics.AddError("Failed to read the supported agent versions", fmt.Sprintf("failed to read the supported agent versions: %s", err))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &edgeUpdateScheduleInfoModel{
		ID:                  types.StringValue("portainer-edge-update-schedule-info"),
		AgentVersions:       edgeStringList(edgeSortedStrings(versions)),
		MinimumAgentVersion: types.StringValue(info.MinAgentVersion),
		UpToDateCount:       types.Int64Value(int64(info.UpToDateCount)),
		OutdatedCount:       types.Int64Value(int64(info.OutdatedCount)),
		HasLocalTimeZone:    types.BoolValue(info.HasLocalTimeZone),
		HasNoLocalTimeZone:  types.BoolValue(info.HasNoLocalTimeZone),
	})...)
}

// --- portainer_edge_update_previous_versions ---

var _ datasource.DataSourceWithConfigure = (*edgeUpdatePreviousVersionsDataSource)(nil)

func newEdgeUpdatePreviousVersionsDataSource() datasource.DataSource {
	return &edgeUpdatePreviousVersionsDataSource{}
}

type edgeUpdatePreviousVersionsDataSource struct{ edgeDataSourceBase }

type edgeUpdatePreviousVersionsModel struct {
	ID               types.String `tfsdk:"id"`
	EnvironmentIDs   types.List   `tfsdk:"environment_ids"`
	EdgeGroupIDs     types.List   `tfsdk:"edge_group_ids"`
	PreviousVersions types.List   `tfsdk:"previous_versions"`
}

var edgeUpdatePreviousVersionAttrTypes = map[string]attr.Type{"endpoint_id": types.StringType, "version": types.StringType}

func (d *edgeUpdatePreviousVersionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_edge_update_previous_versions"
}

func (d *edgeUpdatePreviousVersionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "Always `portainer-edge-update-previous-versions`."},
			"environment_ids": schema.ListAttribute{
				Optional:    true,
				ElementType: types.Int64Type,
				Description: "Only report on these environments.",
			},
			"edge_group_ids": schema.ListAttribute{
				Optional:    true,
				ElementType: types.Int64Type,
				Description: "Only report on the environments in these edge groups.",
			},
			"previous_versions": schema.ListAttribute{
				Computed:    true,
				ElementType: types.ObjectType{AttrTypes: edgeUpdatePreviousVersionAttrTypes},
				Description: "The agent version each environment was on before its last update, which is what a rollback would return it to.",
			},
		},
	}
}

func (d *edgeUpdatePreviousVersionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m edgeUpdatePreviousVersionsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	query := url.Values{}
	for key, list := range map[string]types.List{"environmentIds": m.EnvironmentIDs, "edgeGroupIds": m.EdgeGroupIDs} {
		ids := int64ListToInts(ctx, list, &resp.Diagnostics)
		if len(ids) == 0 {
			continue
		}
		parts := make([]string, 0, len(ids))
		for _, id := range ids {
			parts = append(parts, strconv.Itoa(id))
		}
		query.Set(key, strings.Join(parts, ","))
	}
	listURL := d.client.Endpoint + "/edge_update_schedules/previous_versions"
	if len(query) > 0 {
		listURL += "?" + query.Encode()
	}

	// Portainer answers with a map of environment identifier to version, so
	// the keys are sorted to keep the output stable between two plans.
	var versions map[string]string
	if err := doJSON(ctx, d.client, http.MethodGet, listURL, nil, &versions); err != nil {
		resp.Diagnostics.AddError("Failed to read the previous agent versions", fmt.Sprintf("failed to read the previous agent versions: %s", err))
		return
	}
	endpointIDs := make([]string, 0, len(versions))
	for endpointID := range versions {
		endpointIDs = append(endpointIDs, endpointID)
	}
	sort.Strings(endpointIDs)
	entries := make([]map[string]attr.Value, 0, len(endpointIDs))
	for _, endpointID := range endpointIDs {
		entries = append(entries, map[string]attr.Value{
			"endpoint_id": types.StringValue(endpointID), "version": types.StringValue(versions[endpointID]),
		})
	}

	m.ID = types.StringValue("portainer-edge-update-previous-versions")
	m.PreviousVersions = edgeObjectList(edgeUpdatePreviousVersionAttrTypes, entries, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// --- portainer_edge_update_schedules_active ---

var _ datasource.DataSourceWithConfigure = (*edgeUpdateSchedulesActiveDataSource)(nil)

func newEdgeUpdateSchedulesActiveDataSource() datasource.DataSource {
	return &edgeUpdateSchedulesActiveDataSource{}
}

// edgeUpdateSchedulesActiveDataSource reports which update schedules are
// currently in flight for a set of environments. Portainer takes the
// environment list in a POST body, but the call changes nothing.
type edgeUpdateSchedulesActiveDataSource struct{ edgeDataSourceBase }

type edgeUpdateSchedulesActiveModel struct {
	ID             types.String `tfsdk:"id"`
	EnvironmentIDs types.List   `tfsdk:"environment_ids"`
	Schedules      types.List   `tfsdk:"schedules"`
}

var edgeUpdateActiveAttrTypes = map[string]attr.Type{
	"schedule_id": types.Int64Type, "endpoint_id": types.Int64Type, "edge_stack_id": types.Int64Type, "target_version": types.StringType,
}

func (d *edgeUpdateSchedulesActiveDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_edge_update_schedules_active"
}

func (d *edgeUpdateSchedulesActiveDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "Always `portainer-edge-update-schedules-active`."},
			"environment_ids": schema.ListAttribute{
				Required:    true,
				ElementType: types.Int64Type,
				Description: "Environments to report active update schedules for.",
			},
			"schedules": schema.ListAttribute{
				Computed:    true,
				ElementType: types.ObjectType{AttrTypes: edgeUpdateActiveAttrTypes},
				Description: "The update schedules currently in flight. An empty list means none of the given environments is mid-update.",
			},
		},
	}
}

func (d *edgeUpdateSchedulesActiveDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m edgeUpdateSchedulesActiveModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ids := int64ListToInts(ctx, m.EnvironmentIDs, &resp.Diagnostics)
	if ids == nil {
		ids = []int{}
	}
	payload := map[string]interface{}{"EnvironmentIDs": ids}

	var relations []struct {
		ScheduleID    int    `json:"scheduleId"`
		EnvironmentID int    `json:"environmentId"`
		EdgeStackID   int    `json:"edgeStackId"`
		TargetVersion string `json:"targetVersion"`
	}
	if err := doJSON(ctx, d.client, http.MethodPost, d.client.Endpoint+"/edge_update_schedules/active", payload, &relations); err != nil {
		resp.Diagnostics.AddError("Failed to read the active edge update schedules", fmt.Sprintf("failed to read the active edge update schedules: %s", err))
		return
	}
	entries := make([]map[string]attr.Value, 0, len(relations))
	for _, r := range relations {
		entries = append(entries, map[string]attr.Value{
			"schedule_id": types.Int64Value(int64(r.ScheduleID)), "endpoint_id": types.Int64Value(int64(r.EnvironmentID)),
			"edge_stack_id": types.Int64Value(int64(r.EdgeStackID)), "target_version": types.StringValue(r.TargetVersion),
		})
	}
	m.ID = types.StringValue("portainer-edge-update-schedules-active")
	m.Schedules = edgeObjectList(edgeUpdateActiveAttrTypes, entries, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
