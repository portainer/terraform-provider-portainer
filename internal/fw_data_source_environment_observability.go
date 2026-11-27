package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*environmentLogsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*environmentLogsDataSource)(nil)
	_ datasource.DataSource              = (*environmentMetricsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*environmentMetricsDataSource)(nil)
)

// ---------------------------------------------------------------------------
// portainer_environment_logs
// ---------------------------------------------------------------------------

func newEnvironmentLogsDataSource() datasource.DataSource { return &environmentLogsDataSource{} }

type environmentLogsDataSource struct{ client *APIClient }

type environmentLogsModel struct {
	ID         types.String               `tfsdk:"id"`
	EndpointID types.Int64                `tfsdk:"endpoint_id"`
	From       types.String               `tfsdk:"from"`
	To         types.String               `tfsdk:"to"`
	Namespace  types.String               `tfsdk:"namespace"`
	Kind       types.String               `tfsdk:"kind"`
	Name       types.String               `tfsdk:"name"`
	Search     types.String               `tfsdk:"search"`
	Severity   types.String               `tfsdk:"severity"`
	Skip       types.Int64                `tfsdk:"skip"`
	Limit      types.Int64                `tfsdk:"limit"`
	Logs       []environmentLogEntryModel `tfsdk:"logs"`
}

type environmentLogEntryModel struct {
	Time     types.String `tfsdk:"time"`
	Severity types.String `tfsdk:"severity"`
	Source   types.String `tfsdk:"source"`
	Message  types.String `tfsdk:"message"`
	Labels   types.Map    `tfsdk:"labels"`
}

func (d *environmentLogsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment_logs"
}

func (d *environmentLogsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"endpoint_id": schema.Int64Attribute{Required: true, Description: "Identifier of the environment to query logs for."},
			"from":        schema.StringAttribute{Required: true, Description: "Start of the window to query, as Portainer's observability backend expects it (for example an RFC 3339 timestamp)."},
			"to":          schema.StringAttribute{Required: true, Description: "End of the window to query."},
			"namespace":   schema.StringAttribute{Optional: true, Description: "Only return logs from this namespace."},
			"kind":        schema.StringAttribute{Optional: true, Description: "Only return logs from this resource kind."},
			"name":        schema.StringAttribute{Optional: true, Description: "Only return logs from the resource with this name."},
			"search":      schema.StringAttribute{Optional: true, Description: "Only return log lines matching this text."},
			"severity":    schema.StringAttribute{Optional: true, Description: "Only return log lines of this severity."},
			"skip":        schema.Int64Attribute{Optional: true, Description: "Log lines to skip, for paging through a large window."},
			"limit":       schema.Int64Attribute{Optional: true, Description: "Most log lines to return."},
			// Protocol 5 has no nested attributes: a computed list of objects,
			// as SDKv2 declared it.
			"logs": schema.ListAttribute{
				Computed:    true,
				Description: "The log lines the query returned.",
				ElementType: types.ObjectType{AttrTypes: map[string]attr.Type{
					"time":     types.StringType,
					"severity": types.StringType,
					"source":   types.StringType,
					"message":  types.StringType,
					"labels":   types.MapType{ElemType: types.StringType},
				}},
			},
		},
	}
}

func (d *environmentLogsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *environmentLogsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m environmentLogsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	endpointID := m.EndpointID.ValueInt64()

	query := url.Values{}
	query.Set("from", m.From.ValueString())
	query.Set("to", m.To.ValueString())
	for key, v := range map[string]types.String{
		"namespace": m.Namespace, "kind": m.Kind, "name": m.Name,
		"search": m.Search, "severity": m.Severity,
	} {
		if v.ValueString() != "" {
			query.Set(key, v.ValueString())
		}
	}
	for key, v := range map[string]types.Int64{"skip": m.Skip, "limit": m.Limit} {
		if v.ValueInt64() != 0 {
			query.Set(key, fmt.Sprintf("%d", v.ValueInt64()))
		}
	}

	var response struct {
		Logs []struct {
			Time     string            `json:"time"`
			Severity string            `json:"severity"`
			Source   string            `json:"source"`
			Message  string            `json:"message"`
			Labels   map[string]string `json:"labels"`
		} `json:"logs"`
	}
	logsURL := fmt.Sprintf("%s/observability/environments/%d/logs?%s", d.client.Endpoint, endpointID, query.Encode())
	if err := doJSON(ctx, d.client, http.MethodGet, logsURL, nil, &response); err != nil {
		resp.Diagnostics.AddError("Failed to query environment logs", fmt.Errorf("failed to query the logs of environment %d: %w", endpointID, err).Error())
		return
	}

	m.Logs = make([]environmentLogEntryModel, 0, len(response.Logs))
	for _, entry := range response.Logs {
		m.Logs = append(m.Logs, environmentLogEntryModel{
			Time:     types.StringValue(entry.Time),
			Severity: types.StringValue(entry.Severity),
			Source:   types.StringValue(entry.Source),
			Message:  types.StringValue(entry.Message),
			Labels:   stringMapValue(entry.Labels),
		})
	}
	m.ID = types.StringValue(fmt.Sprintf("portainer-environment-logs-%d", endpointID))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// ---------------------------------------------------------------------------
// portainer_environment_metrics
// ---------------------------------------------------------------------------

func newEnvironmentMetricsDataSource() datasource.DataSource {
	return &environmentMetricsDataSource{}
}

type environmentMetricsDataSource struct{ client *APIClient }

type environmentMetricsModel struct {
	ID          types.String `tfsdk:"id"`
	EndpointID  types.Int64  `tfsdk:"endpoint_id"`
	Metric      types.String `tfsdk:"metric"`
	Aggregation types.String `tfsdk:"aggregation"`
	From        types.String `tfsdk:"from"`
	To          types.String `tfsdk:"to"`
	Namespace   types.String `tfsdk:"namespace"`
	Kind        types.String `tfsdk:"kind"`
	Name        types.String `tfsdk:"name"`
	GroupBy     types.String `tfsdk:"group_by"`
	Data        types.String `tfsdk:"data"`
}

func (d *environmentMetricsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment_metrics"
}

func (d *environmentMetricsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"endpoint_id": schema.Int64Attribute{Required: true, Description: "Identifier of the environment to query metrics for."},
			"metric":      schema.StringAttribute{Required: true, Description: "Metric to aggregate."},
			"aggregation": schema.StringAttribute{Required: true, Description: "How to aggregate it, for example `avg` or `max`."},
			"from":        schema.StringAttribute{Required: true, Description: "Start of the window to query."},
			"to":          schema.StringAttribute{Required: true, Description: "End of the window to query."},
			"namespace":   schema.StringAttribute{Optional: true, Description: "Only aggregate over this namespace."},
			"kind":        schema.StringAttribute{Optional: true, Description: "Only aggregate over this resource kind."},
			"name":        schema.StringAttribute{Optional: true, Description: "Only aggregate over the resource with this name."},
			"group_by":    schema.StringAttribute{Optional: true, Description: "Dimension to group the result by."},
			// The series' shape depends on the metric and the grouping, so it
			// is carried through as JSON rather than flattened into attributes
			// that would only fit one kind of query.
			"data": schema.StringAttribute{
				Computed:    true,
				Description: "The aggregated series as Portainer returns them, encoded as JSON. Decode it with `jsondecode()`; the shape depends on `metric` and `group_by`.",
			},
		},
	}
}

func (d *environmentMetricsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *environmentMetricsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m environmentMetricsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	endpointID := m.EndpointID.ValueInt64()

	query := url.Values{}
	query.Set("metric", m.Metric.ValueString())
	query.Set("aggregation", m.Aggregation.ValueString())
	query.Set("from", m.From.ValueString())
	query.Set("to", m.To.ValueString())
	for key, v := range map[string]types.String{
		"namespace": m.Namespace, "kind": m.Kind, "name": m.Name, "groupBy": m.GroupBy,
	} {
		if v.ValueString() != "" {
			query.Set(key, v.ValueString())
		}
	}

	var response struct {
		Data json.RawMessage `json:"data"`
	}
	metricsURL := fmt.Sprintf("%s/observability/environments/%d/metrics?%s", d.client.Endpoint, endpointID, query.Encode())
	if err := doJSON(ctx, d.client, http.MethodGet, metricsURL, nil, &response); err != nil {
		resp.Diagnostics.AddError("Failed to query environment metrics", fmt.Errorf("failed to query the metrics of environment %d: %w", endpointID, err).Error())
		return
	}

	data := "[]"
	if len(response.Data) > 0 {
		data = string(response.Data)
	}
	m.Data = types.StringValue(data)
	m.ID = types.StringValue(fmt.Sprintf("portainer-environment-metrics-%d-%s", endpointID, m.Metric.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
