package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*recommendationsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*recommendationsDataSource)(nil)
)

// recommendationsDataSource folds Portainer's recommendation listing and its
// summary into one read: the counts are a roll-up of the same list, and
// fetching one without the other is rarely what is wanted.
func newRecommendationsDataSource() datasource.DataSource { return &recommendationsDataSource{} }

type recommendationsDataSource struct{ client *APIClient }

type recommendationsDataSourceModel struct {
	ID              types.String `tfsdk:"id"`
	Category        types.String `tfsdk:"category"`
	Severity        types.String `tfsdk:"severity"`
	Recommendations types.List   `tfsdk:"recommendations"`
	Total           types.Int64  `tfsdk:"total"`
	TotalTypes      types.Int64  `tfsdk:"total_types"`
	SeverityCounts  types.Map    `tfsdk:"severity_counts"`
	CategoryCounts  types.Map    `tfsdk:"category_counts"`
}

type recommendationModel struct {
	TypeID      types.String `tfsdk:"type_id"`
	Title       types.String `tfsdk:"title"`
	Description types.String `tfsdk:"description"`
	Category    types.String `tfsdk:"category"`
	Severity    types.String `tfsdk:"severity"`
	ActionLabel types.String `tfsdk:"action_label"`
	ActionURL   types.String `tfsdk:"action_url"`
}

var recommendationAttrTypes = map[string]attr.Type{
	"type_id":      types.StringType,
	"title":        types.StringType,
	"description":  types.StringType,
	"category":     types.StringType,
	"severity":     types.StringType,
	"action_label": types.StringType,
	"action_url":   types.StringType,
}

func (d *recommendationsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_recommendations"
}

func (d *recommendationsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the data source, always `portainer-recommendations`.",
			},
			"category": schema.StringAttribute{
				Optional:    true,
				Description: "Only list recommendations in this category. Leave unset for all of them.",
			},
			"severity": schema.StringAttribute{
				Optional:    true,
				Description: "Only list recommendations of this severity. Leave unset for all of them.",
			},
			"recommendations": schema.ListAttribute{
				Computed:    true,
				Description: "The recommendations Portainer has for this instance.",
				ElementType: types.ObjectType{AttrTypes: recommendationAttrTypes},
			},
			"total": schema.Int64Attribute{
				Computed:    true,
				Description: "Total recommendations, from Portainer's own summary rather than the filtered listing above.",
			},
			"total_types": schema.Int64Attribute{
				Computed:    true,
				Description: "Distinct recommendation types.",
			},
			"severity_counts": schema.MapAttribute{
				Computed:    true,
				ElementType: types.Int64Type,
				Description: "Recommendation count per severity.",
			},
			"category_counts": schema.MapAttribute{
				Computed:    true,
				ElementType: types.Int64Type,
				Description: "Recommendation count per category.",
			},
		},
	}
}

func (d *recommendationsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *recommendationsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m recommendationsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	query := url.Values{}
	if v := m.Category.ValueString(); v != "" {
		query.Set("category", v)
	}
	if v := m.Severity.ValueString(); v != "" {
		query.Set("severity", v)
	}
	listURL := d.client.Endpoint + "/recommendations"
	if len(query) > 0 {
		listURL += "?" + query.Encode()
	}

	var list []struct {
		TypeID      string `json:"typeId"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Category    string `json:"category"`
		Severity    string `json:"severity"`
		ActionLabel string `json:"actionLabel"`
		ActionURL   string `json:"actionUrl"`
	}
	if err := doJSON(ctx, d.client, http.MethodGet, listURL, nil, &list); err != nil {
		resp.Diagnostics.AddError("Failed to list the recommendations", fmt.Errorf("failed to list the recommendations: %w", err).Error())
		return
	}

	entries := make([]recommendationModel, 0, len(list))
	for _, item := range list {
		entries = append(entries, recommendationModel{
			TypeID: types.StringValue(item.TypeID), Title: types.StringValue(item.Title),
			Description: types.StringValue(item.Description), Category: types.StringValue(item.Category),
			Severity: types.StringValue(item.Severity), ActionLabel: types.StringValue(item.ActionLabel),
			ActionURL: types.StringValue(item.ActionURL),
		})
	}

	// The summary is over every recommendation, not the filtered listing, which
	// is why it is reported separately rather than derived from the list.
	var summary struct {
		Total          int            `json:"total"`
		TotalTypes     int            `json:"totalTypes"`
		SeverityCounts map[string]int `json:"severityCounts"`
		CategoryCounts map[string]int `json:"categoryCounts"`
	}
	if err := doJSON(ctx, d.client, http.MethodGet, d.client.Endpoint+"/recommendations/summary", nil, &summary); err != nil {
		resp.Diagnostics.AddError("Failed to read the recommendation summary", fmt.Errorf("failed to read the recommendation summary: %w", err).Error())
		return
	}

	recs, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: recommendationAttrTypes}, entries)
	resp.Diagnostics.Append(diags...)
	m.Recommendations = recs
	m.Total = types.Int64Value(int64(summary.Total))
	m.TotalTypes = types.Int64Value(int64(summary.TotalTypes))
	m.SeverityCounts = recommendationCountMap(summary.SeverityCounts)
	m.CategoryCounts = recommendationCountMap(summary.CategoryCounts)
	m.ID = types.StringValue("portainer-recommendations")
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// recommendationCountMap turns a map of counts into a map(number); empty,
// not null, when Portainer sends none, as SDKv2 stored it.
func recommendationCountMap(source map[string]int) types.Map {
	elems := make(map[string]attr.Value, len(source))
	for k, v := range source {
		elems[k] = types.Int64Value(int64(v))
	}
	return types.MapValueMust(types.Int64Type, elems)
}
