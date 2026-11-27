package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// --- portainer_edge_job_tasks ---

var _ datasource.DataSourceWithConfigure = (*edgeJobTasksDataSource)(nil)

func newEdgeJobTasksDataSource() datasource.DataSource { return &edgeJobTasksDataSource{} }

type edgeJobTasksDataSource struct{ edgeDataSourceBase }

type edgeJobTasksModel struct {
	ID        types.String `tfsdk:"id"`
	EdgeJobID types.Int64  `tfsdk:"edge_job_id"`
	Tasks     types.List   `tfsdk:"tasks"`
}

var edgeJobTaskAttrTypes = map[string]attr.Type{
	"id": types.StringType, "endpoint_id": types.Int64Type, "endpoint_name": types.StringType,
	"logs_status": types.Int64Type, "logs_collected": types.BoolType,
}

func (d *edgeJobTasksDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_edge_job_tasks"
}

func (d *edgeJobTasksDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "`<edge_job_id>/tasks`."},
			"edge_job_id": schema.Int64Attribute{
				Required:    true,
				Description: "Identifier of the edge job whose tasks are listed.",
			},
			"tasks": schema.ListAttribute{
				Computed:    true,
				ElementType: types.ObjectType{AttrTypes: edgeJobTaskAttrTypes},
				Description: "One task per environment the edge job runs on.",
			},
		},
	}
}

func (d *edgeJobTasksDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m edgeJobTasksModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	jobID := m.EdgeJobID.ValueInt64()

	var list []struct {
		ID           string `json:"Id"`
		EndpointID   int    `json:"EndpointId"`
		EndpointName string `json:"EndpointName"`
		LogsStatus   int    `json:"LogsStatus"`
	}
	if err := doJSON(ctx, d.client, http.MethodGet, fmt.Sprintf("%s/edge_jobs/%d/tasks", d.client.Endpoint, jobID), nil, &list); err != nil {
		resp.Diagnostics.AddError("Failed to list the edge job tasks", fmt.Sprintf("failed to list the tasks of edge job %d: %s", jobID, err))
		return
	}

	// EdgeJobLogsStatusCollected is 3 in Portainer's enum; anything below means
	// the logs are not there to read yet.
	const logsCollected = 3

	tasks := make([]map[string]attr.Value, 0, len(list))
	for _, t := range list {
		tasks = append(tasks, map[string]attr.Value{
			"id": types.StringValue(t.ID), "endpoint_id": types.Int64Value(int64(t.EndpointID)),
			"endpoint_name": types.StringValue(t.EndpointName), "logs_status": types.Int64Value(int64(t.LogsStatus)),
			"logs_collected": types.BoolValue(t.LogsStatus == logsCollected),
		})
	}
	m.Tasks = edgeObjectList(edgeJobTaskAttrTypes, tasks, &resp.Diagnostics)
	m.ID = types.StringValue(strconv.FormatInt(jobID, 10) + "/tasks")
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// --- portainer_edge_job_task_logs ---

var _ datasource.DataSourceWithConfigure = (*edgeJobTaskLogsDataSource)(nil)

func newEdgeJobTaskLogsDataSource() datasource.DataSource { return &edgeJobTaskLogsDataSource{} }

type edgeJobTaskLogsDataSource struct{ edgeDataSourceBase }

type edgeJobTaskLogsDataModel struct {
	ID        types.String `tfsdk:"id"`
	EdgeJobID types.Int64  `tfsdk:"edge_job_id"`
	TaskID    types.String `tfsdk:"task_id"`
	Logs      types.String `tfsdk:"logs"`
}

func (d *edgeJobTaskLogsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_edge_job_task_logs"
}

func (d *edgeJobTaskLogsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "`<edge_job_id>/<task_id>/logs`."},
			"edge_job_id": schema.Int64Attribute{
				Required:    true,
				Description: "Identifier of the edge job.",
			},
			"task_id": schema.StringAttribute{
				Required:    true,
				Description: "Identifier of the task, from the `portainer_edge_job_tasks` data source.",
			},
			"logs": schema.StringAttribute{
				Computed:    true,
				Description: "Collected log output of the task. Empty until collection has been requested and the agent has reported back — see `portainer_edge_job_task_logs` (the resource) for triggering it.",
			},
		},
	}
}

func (d *edgeJobTaskLogsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m edgeJobTaskLogsDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	jobID, taskID := m.EdgeJobID.ValueInt64(), m.TaskID.ValueString()

	var body struct {
		FileContent string `json:"FileContent"`
	}
	url := fmt.Sprintf("%s/edge_jobs/%d/tasks/%s/logs", d.client.Endpoint, jobID, taskID)
	if err := doJSON(ctx, d.client, http.MethodGet, url, nil, &body); err != nil && !isAPINotFound(err) {
		resp.Diagnostics.AddError("Failed to read the task logs", fmt.Sprintf("failed to read the logs of task %s of edge job %d: %s", taskID, jobID, err))
		return
	}
	// Logs that have not been collected yet are absent rather than an error
	// worth failing a plan over: body stays empty on a 404.
	m.Logs = types.StringValue(body.FileContent)
	m.ID = types.StringValue(fmt.Sprintf("%d/%s/logs", jobID, taskID))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
