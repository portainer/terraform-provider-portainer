package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*gitopsWorkflowsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*gitopsWorkflowsDataSource)(nil)
)

func newGitopsWorkflowsDataSource() datasource.DataSource { return &gitopsWorkflowsDataSource{} }

type gitopsWorkflowsDataSource struct{ client *APIClient }

type gitopsWorkflowsModel struct {
	ID          types.String `tfsdk:"id"`
	Search      types.String `tfsdk:"search"`
	Sort        types.String `tfsdk:"sort"`
	Order       types.String `tfsdk:"order"`
	Start       types.Int64  `tfsdk:"start"`
	Limit       types.Int64  `tfsdk:"limit"`
	EndpointIDs types.List   `tfsdk:"endpoint_ids"`
	Status      types.String `tfsdk:"status"`
	Type        types.String `tfsdk:"type"`
	Platform    types.String `tfsdk:"platform"`
	Workflows   types.List   `tfsdk:"workflows"`
	Summary     types.List   `tfsdk:"summary"`
}

func (d *gitopsWorkflowsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gitops_workflows"
}

func (d *gitopsWorkflowsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"search": schema.StringAttribute{
				Optional:    true,
				Description: "Free-text filter matched against the workflow name.",
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
			"endpoint_ids": schema.ListAttribute{
				Optional:    true,
				ElementType: types.Int64Type,
				Description: "Only return workflows deploying to these environment identifiers.",
			},
			"status": schema.StringAttribute{
				Optional:    true,
				Validators:  []validator.String{stringvalidator.OneOf("healthy", "syncing", "error", "paused", "unknown", "drifted")},
				Description: "Only return workflows in this status. `drifted` requires Portainer Business Edition 3.0 or later.",
			},
			"type": schema.StringAttribute{
				Optional:    true,
				Validators:  []validator.String{stringvalidator.OneOf("stack", "edgeStack", "gitopsDeployment")},
				Description: "Only return workflows with at least one artifact of this type. `gitopsDeployment`, an artifact on the `reconcile` engine, requires Portainer Business Edition 3.0 or later.",
			},
			"platform": schema.StringAttribute{
				Optional:    true,
				Validators:  []validator.String{stringvalidator.OneOf("dockerStandalone", "dockerSwarm", "kubernetes")},
				Description: "Only return workflows deploying to this platform.",
			},
			"workflows": schema.ListAttribute{
				Computed:    true,
				ElementType: types.ObjectType{AttrTypes: gitopsWorkflowAttrTypes},
				Description: "GitOps workflows matching the query.",
			},
			"summary": gitopsDSSummaryAttribute("GitOps workflows"),
		},
	}
}

func (d *gitopsWorkflowsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *gitopsWorkflowsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data gitopsWorkflowsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	client := d.client

	q := url.Values{}
	for param, v := range map[string]types.String{
		"search": data.Search, "sort": data.Sort, "order": data.Order,
		"status": data.Status, "type": data.Type, "platform": data.Platform,
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
	if ids := int64ListToInts(ctx, data.EndpointIDs, &resp.Diagnostics); len(ids) > 0 {
		parts := make([]string, len(ids))
		for i, id := range ids {
			parts[i] = strconv.Itoa(id)
		}
		q.Set("endpointIds", strings.Join(parts, ","))
	}
	if resp.Diagnostics.HasError() {
		return
	}
	query := ""
	if len(q) > 0 {
		query = "?" + q.Encode()
	}

	var list []gitopsWorkflow
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/gitops/workflows"+query, nil, &list); err != nil {
		gitopsDSAddError(&resp.Diagnostics, fmt.Errorf("failed to list GitOps workflows: %w", err))
		return
	}

	workflows := make([]attr.Value, len(list))
	for i, w := range list {
		workflows[i] = types.ObjectValueMust(gitopsWorkflowAttrTypes, w.attrValues())
	}

	// The summary endpoint is unfiltered and always describes every workflow.
	var summary gitopsStatusSummary
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/gitops/workflows/summary", nil, &summary); err != nil {
		gitopsDSAddError(&resp.Diagnostics, fmt.Errorf("failed to read the GitOps workflow summary: %w", err))
		return
	}

	data.Workflows = types.ListValueMust(types.ObjectType{AttrTypes: gitopsWorkflowAttrTypes}, workflows)
	data.Summary = summary.toList()
	data.ID = types.StringValue("gitops-workflows-" + strconv.FormatInt(gitopsDSTimestamp(), 10))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
