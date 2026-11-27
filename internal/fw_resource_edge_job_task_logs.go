package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource              = (*edgeJobTaskLogsResource)(nil)
	_ resource.ResourceWithConfigure = (*edgeJobTaskLogsResource)(nil)
)

func newEdgeJobTaskLogsResource() resource.Resource { return &edgeJobTaskLogsResource{} }

// edgeJobTaskLogsResource asks for the logs of an edge job task on create and
// clears them on destroy.
type edgeJobTaskLogsResource struct{ client *APIClient }

type edgeJobTaskLogsModel struct {
	ID        types.String `tfsdk:"id"`
	EdgeJobID types.Int64  `tfsdk:"edge_job_id"`
	TaskID    types.String `tfsdk:"task_id"`
}

func (r *edgeJobTaskLogsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_edge_job_task_logs"
}

func (r *edgeJobTaskLogsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the log collection request, `<edge_job_id>/<task_id>/logs`.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"edge_job_id": schema.Int64Attribute{
				Required:      true,
				Description:   "Identifier of the edge job.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"task_id": schema.StringAttribute{
				Required:      true,
				Description:   "Identifier of the task, from the `portainer_edge_job_tasks` data source.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
	}
}

func (r *edgeJobTaskLogsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *edgeJobTaskLogsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan edgeJobTaskLogsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	jobID, taskID := plan.EdgeJobID.ValueInt64(), plan.TaskID.ValueString()

	// This only asks for collection; the edge agent uploads the logs on its next
	// check-in, so they are not readable the moment this returns.
	url := fmt.Sprintf("%s/edge_jobs/%d/tasks/%s/logs", r.client.Endpoint, jobID, taskID)
	if err := doJSON(ctx, r.client, http.MethodPost, url, nil, nil); err != nil {
		resp.Diagnostics.AddError("Failed to request log collection", fmt.Sprintf("failed to request log collection for task %s of edge job %d: %s", taskID, jobID, err))
		return
	}
	plan.ID = types.StringValue(fmt.Sprintf("%d/%s/logs", jobID, taskID))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read keeps the state, as the SDKv2 version (a no-op read) did.
func (r *edgeJobTaskLogsResource) Read(context.Context, resource.ReadRequest, *resource.ReadResponse) {
}

// Update is never called: every attribute requires replacement.
func (r *edgeJobTaskLogsResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Edge job task logs cannot be updated", "Every attribute of portainer_edge_job_task_logs forces a new resource.")
}

func (r *edgeJobTaskLogsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state edgeJobTaskLogsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	jobID, taskID := state.EdgeJobID.ValueInt64(), state.TaskID.ValueString()
	url := fmt.Sprintf("%s/edge_jobs/%d/tasks/%s/logs", r.client.Endpoint, jobID, taskID)
	if err := doJSON(ctx, r.client, http.MethodDelete, url, nil, nil); err != nil && !isAPINotFound(err) {
		resp.Diagnostics.AddError("Failed to clear the task logs", fmt.Sprintf("failed to clear the logs of task %s of edge job %d: %s", taskID, jobID, err))
	}
}
