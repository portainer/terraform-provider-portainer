package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*userActivityDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*userActivityDataSource)(nil)
)

func newUserActivityDataSource() datasource.DataSource { return &userActivityDataSource{} }

type userActivityDataSource struct{ client *APIClient }

type userActivityDataSourceModel struct {
	ID           types.String                `tfsdk:"id"`
	LogType      types.String                `tfsdk:"log_type"`
	Keyword      types.String                `tfsdk:"keyword"`
	Username     types.List                  `tfsdk:"username"`
	Context      types.List                  `tfsdk:"context"`
	Before       types.Int64                 `tfsdk:"before"`
	After        types.Int64                 `tfsdk:"after"`
	SortBy       types.String                `tfsdk:"sort_by"`
	SortDesc     types.Bool                  `tfsdk:"sort_desc"`
	Offset       types.Int64                 `tfsdk:"offset"`
	Limit        types.Int64                 `tfsdk:"limit"`
	ActivityLogs []userActivityLogEntryModel `tfsdk:"activity_logs"`
	AuthLogs     []userAuthLogEntryModel     `tfsdk:"auth_logs"`
	TotalCount   types.Int64                 `tfsdk:"total_count"`
}

type userActivityLogEntryModel struct {
	ID        int64  `tfsdk:"id"`
	Timestamp int64  `tfsdk:"timestamp"`
	Username  string `tfsdk:"username"`
	Action    string `tfsdk:"action"`
	Context   string `tfsdk:"context"`
}

type userAuthLogEntryModel struct {
	ID        int64  `tfsdk:"id"`
	Timestamp int64  `tfsdk:"timestamp"`
	Username  string `tfsdk:"username"`
	Type      int64  `tfsdk:"type"`
	Origin    string `tfsdk:"origin"`
	Context   int64  `tfsdk:"context"`
}

func (d *userActivityDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_activity"
}

// Schema: log_type, sort_desc, offset and limit had SDKv2 defaults. Data
// sources of the framework have none, so they are optional+computed and Read
// fills in the same default when the configuration leaves them out.
func (d *userActivityDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "Synthetic identifier of the read (a Unix timestamp)."},
			"log_type": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Type of logs to retrieve: 'activity' for user activity logs, 'auth' for authentication logs.",
			},
			"keyword": schema.StringAttribute{
				Optional:    true,
				Description: "Filter logs by keyword.",
			},
			"username": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Filter by usernames (only for activity logs).",
			},
			"context": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Filter by context (only for activity logs).",
			},
			"before": schema.Int64Attribute{
				Optional:    true,
				Description: "Return results before this unix timestamp.",
			},
			"after": schema.Int64Attribute{
				Optional:    true,
				Description: "Return results after this unix timestamp.",
			},
			"sort_by": schema.StringAttribute{
				Optional:    true,
				Description: "Sort results by this column.",
			},
			"sort_desc": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Sort in descending order.",
			},
			"offset": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Pagination offset.",
			},
			"limit": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Limit number of results.",
			},
			"activity_logs": schema.ListAttribute{
				Computed:    true,
				Description: "List of user activity log entries returned by Portainer when `log_type` is set to `activity`.",
				ElementType: types.ObjectType{AttrTypes: map[string]attr.Type{
					"id":        types.Int64Type,
					"timestamp": types.Int64Type,
					"username":  types.StringType,
					"action":    types.StringType,
					"context":   types.StringType,
				}},
			},
			"auth_logs": schema.ListAttribute{
				Computed:    true,
				Description: "List of authentication log entries returned by Portainer when `log_type` is set to `auth`.",
				ElementType: types.ObjectType{AttrTypes: map[string]attr.Type{
					"id":        types.Int64Type,
					"timestamp": types.Int64Type,
					"username":  types.StringType,
					"type":      types.Int64Type,
					"origin":    types.StringType,
					"context":   types.Int64Type,
				}},
			},
			"total_count": schema.Int64Attribute{
				Computed:    true,
				Description: "Total count of matching logs, as reported by Portainer.",
			},
		},
	}
}

func (d *userActivityDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *userActivityDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data userActivityDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The SDKv2 defaults.
	if data.LogType.IsNull() {
		data.LogType = types.StringValue("activity")
	}
	if data.SortDesc.IsNull() {
		data.SortDesc = types.BoolValue(false)
	}
	if data.Offset.IsNull() {
		data.Offset = types.Int64Value(0)
	}
	if data.Limit.IsNull() {
		data.Limit = types.Int64Value(100)
	}

	logType := data.LogType.ValueString()
	var path string
	switch logType {
	case "activity":
		path = "/useractivity/logs"
	case "auth":
		path = "/useractivity/authlogs"
	default:
		resp.Diagnostics.AddError("Failed to read user activity", fmt.Sprintf("invalid log_type %q: must be 'activity' or 'auth'", logType))
		return
	}

	// Query parameters; zero values are left out, as SDKv2's GetOk did.
	params := []string{}
	if v := data.Offset.ValueInt64(); v != 0 {
		params = append(params, fmt.Sprintf("offset=%d", v))
	}
	if v := data.Limit.ValueInt64(); v != 0 {
		params = append(params, fmt.Sprintf("limit=%d", v))
	}
	if v := data.Before.ValueInt64(); v != 0 {
		params = append(params, fmt.Sprintf("before=%d", v))
	}
	if v := data.After.ValueInt64(); v != 0 {
		params = append(params, fmt.Sprintf("after=%d", v))
	}
	if v := data.SortBy.ValueString(); v != "" {
		params = append(params, fmt.Sprintf("sortBy=%s", url.QueryEscape(v)))
	}
	if data.SortDesc.ValueBool() {
		params = append(params, "sortDesc=true")
	}
	if v := data.Keyword.ValueString(); v != "" {
		params = append(params, fmt.Sprintf("keyword=%s", url.QueryEscape(v)))
	}

	// Activity-log-only filters
	if logType == "activity" {
		if vals := userActivityStrings(ctx, data.Username, resp); len(vals) > 0 {
			params = append(params, fmt.Sprintf("username=%s", url.QueryEscape(strings.Join(vals, ","))))
		}
		if vals := userActivityStrings(ctx, data.Context, resp); len(vals) > 0 {
			params = append(params, fmt.Sprintf("context=%s", url.QueryEscape(strings.Join(vals, ","))))
		}
		if resp.Diagnostics.HasError() {
			return
		}
	}

	if len(params) > 0 {
		path = path + "?" + strings.Join(params, "&")
	}

	// Both lists are always set (the one not requested empty), as SDKv2
	// stored them.
	data.ActivityLogs = []userActivityLogEntryModel{}
	data.AuthLogs = []userAuthLogEntryModel{}

	if logType == "activity" {
		var result struct {
			Logs []struct {
				ID        int64  `json:"id"`
				Timestamp int64  `json:"timestamp"`
				Username  string `json:"username"`
				Action    string `json:"action"`
				Context   string `json:"context"`
			} `json:"logs"`
			TotalCount int64 `json:"totalCount"`
		}
		if err := doJSON(ctx, d.client, http.MethodGet, d.client.Endpoint+path, nil, &result); err != nil {
			resp.Diagnostics.AddError("Failed to read user activity", fmt.Errorf("failed to list user activity logs: %w", err).Error())
			return
		}
		for _, l := range result.Logs {
			data.ActivityLogs = append(data.ActivityLogs, userActivityLogEntryModel{
				ID: l.ID, Timestamp: l.Timestamp, Username: l.Username, Action: l.Action, Context: l.Context,
			})
		}
		data.TotalCount = types.Int64Value(result.TotalCount)
	} else {
		var raw json.RawMessage
		if err := doJSON(ctx, d.client, http.MethodGet, d.client.Endpoint+path, nil, &raw); err != nil {
			resp.Diagnostics.AddError("Failed to read user activity", fmt.Errorf("failed to list user activity logs: %w", err).Error())
			return
		}
		logs, total, err := decodeAuthLogs(raw)
		if err != nil {
			resp.Diagnostics.AddError("Failed to read user activity", fmt.Errorf("failed to decode the authentication logs: %w", err).Error())
			return
		}
		for _, l := range logs {
			data.AuthLogs = append(data.AuthLogs, userAuthLogEntryModel(l))
		}
		data.TotalCount = types.Int64Value(total)
	}

	data.ID = types.StringValue(strconv.FormatInt(time.Now().Unix(), 10))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// authLogEntry is one entry of GET /useractivity/authlogs.
type authLogEntry struct {
	ID        int64  `json:"id"`
	Timestamp int64  `json:"timestamp"`
	Username  string `json:"username"`
	Type      int64  `json:"type"`
	Origin    string `json:"origin"`
	Context   int64  `json:"context"`
}

// decodeAuthLogs decodes GET /useractivity/authlogs. Portainer (seen on BE
// 3.1.0) returns the paginated object {"logs": [...], "totalCount": N}, like
// /useractivity/logs, although every published spec up to 3.1.0 documents a
// bare array; the array is still accepted (its total is its length).
func decodeAuthLogs(raw json.RawMessage) ([]authLogEntry, int64, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, 0, nil
	}
	if trimmed[0] == '[' {
		var logs []authLogEntry
		if err := json.Unmarshal(trimmed, &logs); err != nil {
			return nil, 0, err
		}
		return logs, int64(len(logs)), nil
	}
	var page struct {
		Logs       []authLogEntry `json:"logs"`
		TotalCount int64          `json:"totalCount"`
	}
	if err := json.Unmarshal(trimmed, &page); err != nil {
		return nil, 0, err
	}
	return page.Logs, page.TotalCount, nil
}

// userActivityStrings reads a list(string) filter (nil when null).
func userActivityStrings(ctx context.Context, l types.List, resp *datasource.ReadResponse) []string {
	if l.IsNull() || l.IsUnknown() {
		return nil
	}
	var vals []string
	resp.Diagnostics.Append(l.ElementsAs(ctx, &vals, false)...)
	return vals
}
