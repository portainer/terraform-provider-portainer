package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*gitopsSourcesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*gitopsSourcesDataSource)(nil)
)

func newGitopsSourcesDataSource() datasource.DataSource { return &gitopsSourcesDataSource{} }

type gitopsSourcesDataSource struct{ client *APIClient }

type gitopsSourcesModel struct {
	ID      types.String `tfsdk:"id"`
	Search  types.String `tfsdk:"search"`
	Sort    types.String `tfsdk:"sort"`
	Order   types.String `tfsdk:"order"`
	Start   types.Int64  `tfsdk:"start"`
	Limit   types.Int64  `tfsdk:"limit"`
	Status  types.String `tfsdk:"status"`
	Type    types.String `tfsdk:"type"`
	Sources types.List   `tfsdk:"sources"`
	Summary types.List   `tfsdk:"summary"`
}

var gitopsSourcesDSItemAttrTypes = map[string]attr.Type{
	"id": types.Int64Type, "name": types.StringType, "url": types.StringType, "type": types.StringType,
	"status": types.StringType, "status_error": types.StringType, "interval": types.StringType,
	"last_sync": types.Int64Type, "used_by": types.Int64Type, "environments": types.Int64Type,
}

func (d *gitopsSourcesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gitops_sources"
}

func (d *gitopsSourcesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"search": schema.StringAttribute{
				Optional:    true,
				Description: "Free-text filter matched against the source name and URL.",
			},
			"sort": schema.StringAttribute{
				Optional:    true,
				Description: "Field to sort the results by, as accepted by the Portainer API (for example `name`).",
			},
			"order": schema.StringAttribute{
				Optional:    true,
				Validators:  []validator.String{stringvalidator.OneOf("asc", "desc")},
				Description: "Sort direction, `asc` or `desc`.",
			},
			"start": schema.Int64Attribute{
				Optional:    true,
				Description: "Zero-based index of the first result to return, for paging through a large list.",
			},
			"limit": schema.Int64Attribute{
				Optional:    true,
				Description: "Maximum number of results to return. Leave unset to let Portainer decide.",
			},
			"status": schema.StringAttribute{
				Optional:    true,
				Validators:  []validator.String{stringvalidator.OneOf("healthy", "syncing", "error", "paused", "unknown")},
				Description: "Only return sources in this sync status.",
			},
			"type": schema.StringAttribute{
				Optional:    true,
				Validators:  []validator.String{stringvalidator.OneOf("git", "helm", "oci")},
				Description: "Only return sources of this type.",
			},
			"sources": schema.ListAttribute{
				Computed:    true,
				ElementType: types.ObjectType{AttrTypes: gitopsSourcesDSItemAttrTypes},
				Description: "GitOps sources matching the query.",
			},
			"summary": gitopsDSSummaryAttribute("GitOps sources"),
		},
	}
}

func (d *gitopsSourcesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *gitopsSourcesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data gitopsSourcesModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	client := d.client

	q := url.Values{}
	for param, v := range map[string]types.String{
		"search": data.Search, "sort": data.Sort, "order": data.Order, "status": data.Status, "type": data.Type,
	} {
		if s, ok := gitopsDSString(v); ok {
			q.Set(param, s)
		}
	}
	for param, v := range map[string]types.Int64{"start": data.Start, "limit": data.Limit} {
		if n, ok := gitopsDSInt(v); ok {
			q.Set(param, strconv.Itoa(n))
		}
	}
	query := ""
	if len(q) > 0 {
		query = "?" + q.Encode()
	}

	var list []struct {
		ID           int    `json:"id"`
		Name         string `json:"name"`
		URL          string `json:"url"`
		Type         string `json:"type"`
		Status       string `json:"status"`
		Error        string `json:"error"`
		Interval     string `json:"interval"`
		LastSync     int    `json:"lastSync"`
		UsedBy       int    `json:"usedBy"`
		Environments int    `json:"environments"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/gitops/sources"+query, nil, &list); err != nil {
		gitopsDSAddError(&resp.Diagnostics, fmt.Errorf("failed to list GitOps sources: %w", err))
		return
	}

	sources := make([]attr.Value, len(list))
	for i, s := range list {
		sources[i] = types.ObjectValueMust(gitopsSourcesDSItemAttrTypes, map[string]attr.Value{
			"id": types.Int64Value(int64(s.ID)), "name": types.StringValue(s.Name),
			"url": types.StringValue(s.URL), "type": types.StringValue(s.Type),
			"status": types.StringValue(s.Status), "status_error": types.StringValue(s.Error),
			"interval": types.StringValue(s.Interval), "last_sync": types.Int64Value(int64(s.LastSync)),
			"used_by": types.Int64Value(int64(s.UsedBy)), "environments": types.Int64Value(int64(s.Environments)),
		})
	}

	// The summary is its own endpoint and is unfiltered: it always describes
	// every source, not just the ones matching the query above.
	var summary gitopsStatusSummary
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/gitops/sources/summary", nil, &summary); err != nil {
		gitopsDSAddError(&resp.Diagnostics, fmt.Errorf("failed to read the GitOps source summary: %w", err))
		return
	}

	data.Sources = types.ListValueMust(types.ObjectType{AttrTypes: gitopsSourcesDSItemAttrTypes}, sources)
	data.Summary = summary.toList()
	data.ID = types.StringValue("gitops-sources-" + strconv.FormatInt(gitopsDSTimestamp(), 10))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
