package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type EdgeUpdateSchedulePayload struct {
	Name          string `json:"name"`
	AgentImage    string `json:"agentImage"`
	UpdaterImage  string `json:"updaterImage"`
	RegistryID    int    `json:"registryID"`
	ScheduledTime string `json:"scheduledTime"`
	GroupIDs      []int  `json:"groupIDs"`
	Type          int    `json:"type"`
}

type EdgeUpdateScheduleResponse struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	AgentImage    string `json:"agentImage"`
	UpdaterImage  string `json:"updaterImage"`
	RegistryID    int    `json:"registryId"`
	ScheduledTime string `json:"scheduledTime"`
	EdgeGroupIds  []int  `json:"edgeGroupIds"`
	Type          int    `json:"type"`
	Status        int    `json:"status"`
	StatusMessage string `json:"statusMessage"`
	Version       string `json:"version"`
}

var (
	_ resource.Resource                = (*edgeUpdateSchedulesResource)(nil)
	_ resource.ResourceWithConfigure   = (*edgeUpdateSchedulesResource)(nil)
	_ resource.ResourceWithImportState = (*edgeUpdateSchedulesResource)(nil)
)

func newEdgeUpdateSchedulesResource() resource.Resource { return &edgeUpdateSchedulesResource{} }

type edgeUpdateSchedulesResource struct{ client *APIClient }

type edgeUpdateSchedulesModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	AgentImage    types.String `tfsdk:"agent_image"`
	UpdaterImage  types.String `tfsdk:"updater_image"`
	RegistryID    types.Int64  `tfsdk:"registry_id"`
	ScheduledTime types.String `tfsdk:"scheduled_time"`
	GroupIDs      types.List   `tfsdk:"group_ids"`
	Type          types.Int64  `tfsdk:"type"`
}

func (r *edgeUpdateSchedulesResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_edge_update_schedules"
}

func (r *edgeUpdateSchedulesResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the edge update schedule.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name":           schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.LengthAtLeast(1)}, Description: "Name of the Portainer edge update schedule."},
			"agent_image":    schema.StringAttribute{Required: true, Description: "Container image used as the target Portainer Edge agent image after the update runs."},
			"updater_image":  schema.StringAttribute{Required: true, Description: "Container image used to perform the Edge agent update."},
			"registry_id":    schema.Int64Attribute{Required: true, Description: "Identifier of the Portainer registry from which the agent and updater images are pulled."},
			"scheduled_time": schema.StringAttribute{Required: true, Description: "Time in RFC3339 format"},
			"group_ids":      schema.ListAttribute{Required: true, ElementType: types.Int64Type, Description: "List of edge group identifiers targeted by the update schedule."},
			"type":           schema.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.Between(0, 1)}, Description: "0 = update, 1 = rollback"},
		},
	}
}

func (r *edgeUpdateSchedulesResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func edgeUpdateSchedulePayloadFrom(ctx context.Context, m *edgeUpdateSchedulesModel, diags *diag.Diagnostics) EdgeUpdateSchedulePayload {
	payload := EdgeUpdateSchedulePayload{
		Name:          m.Name.ValueString(),
		AgentImage:    m.AgentImage.ValueString(),
		UpdaterImage:  m.UpdaterImage.ValueString(),
		RegistryID:    int(m.RegistryID.ValueInt64()),
		ScheduledTime: m.ScheduledTime.ValueString(),
		Type:          int(m.Type.ValueInt64()),
	}
	// Left nil (null in JSON) for an empty list, as the SDKv2 version sent it.
	payload.GroupIDs = append(payload.GroupIDs, int64ListToInts(ctx, m.GroupIDs, diags)...)
	return payload
}

func (r *edgeUpdateSchedulesResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan edgeUpdateSchedulesModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	payload := edgeUpdateSchedulePayloadFrom(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var response EdgeUpdateScheduleResponse
	if err := doJSON(ctx, r.client, http.MethodPost, fmt.Sprintf("%s/edge_update_schedules", r.client.Endpoint), payload, &response); err != nil {
		resp.Diagnostics.AddError("Failed to create edge update schedule", fmt.Sprintf("failed to create edge update schedule: %s", err))
		return
	}
	// Like the SDKv2 version, the schedule is not read back after creation.
	plan.ID = types.StringValue(strconv.Itoa(response.ID))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *edgeUpdateSchedulesResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state edgeUpdateSchedulesModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.readInto(ctx, &state, &resp.State, &resp.Diagnostics)
}

func (r *edgeUpdateSchedulesResource) readInto(ctx context.Context, m *edgeUpdateSchedulesModel, state *tfsdk.State, diags *diag.Diagnostics) {
	var data EdgeUpdateScheduleResponse
	if err := doJSON(ctx, r.client, http.MethodGet, fmt.Sprintf("%s/edge_update_schedules/%s", r.client.Endpoint, m.ID.ValueString()), nil, &data); err != nil {
		if isAPINotFound(err) {
			state.RemoveResource(ctx)
			return
		}
		diags.AddError("Failed to read edge update schedule", fmt.Sprintf("failed to read edge update schedule: %s", err))
		return
	}
	m.Name = types.StringValue(data.Name)
	m.AgentImage = types.StringValue(data.AgentImage)
	m.UpdaterImage = types.StringValue(data.UpdaterImage)
	m.RegistryID = types.Int64Value(int64(data.RegistryID))
	m.ScheduledTime = types.StringValue(data.ScheduledTime)
	m.GroupIDs = int64ListValue(data.EdgeGroupIds)
	m.Type = types.Int64Value(int64(data.Type))
	diags.Append(state.Set(ctx, m)...)
}

func (r *edgeUpdateSchedulesResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan edgeUpdateSchedulesModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	payload := edgeUpdateSchedulePayloadFrom(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	// Portainer updates a schedule with a POST to its URL.
	if err := doJSON(ctx, r.client, http.MethodPost, fmt.Sprintf("%s/edge_update_schedules/%s", r.client.Endpoint, plan.ID.ValueString()), payload, nil); err != nil {
		resp.Diagnostics.AddError("Failed to update edge update schedule", fmt.Sprintf("failed to update edge update schedule: %s", err))
		return
	}
	r.readInto(ctx, &plan, &resp.State, &resp.Diagnostics)
}

func (r *edgeUpdateSchedulesResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state edgeUpdateSchedulesModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := doJSON(ctx, r.client, http.MethodDelete, fmt.Sprintf("%s/edge_update_schedules/%s", r.client.Endpoint, state.ID.ValueString()), nil, nil); err != nil && !isAPINotFound(err) {
		resp.Diagnostics.AddError("Failed to delete edge update schedule", fmt.Sprintf("failed to delete edge update schedule: %s", err))
	}
}

func (r *edgeUpdateSchedulesResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
