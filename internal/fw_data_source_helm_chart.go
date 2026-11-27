package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*helmChartDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*helmChartDataSource)(nil)
)

func newHelmChartDataSource() datasource.DataSource { return &helmChartDataSource{} }

type helmChartDataSource struct{ client *APIClient }

type helmChartDataSourceModel struct {
	ID      types.String `tfsdk:"id"`
	Repo    types.String `tfsdk:"repo"`
	Chart   types.String `tfsdk:"chart"`
	Command types.String `tfsdk:"command"`
	Version types.String `tfsdk:"version"`
	Content types.String `tfsdk:"content"`
}

func (d *helmChartDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_helm_chart"
}

func (d *helmChartDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the data source, built from the command, repository and chart.",
			},
			"repo": schema.StringAttribute{
				Required:    true,
				Description: "URL of the Helm repository to read from.",
			},
			"chart": schema.StringAttribute{
				Optional:    true,
				Description: "Chart to inspect. Leave unset to fetch the repository index instead.",
			},
			// Optional+computed so that, as with SDKv2's default, the state
			// holds "values" when the argument is left out.
			"command": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Validators:  []validator.String{stringvalidator.OneOf("values", "readme", "chart")},
				Description: "What to fetch for `chart`: its default `values`, its `readme`, or the `chart` metadata. Ignored when `chart` is unset.",
			},
			"version": schema.StringAttribute{
				Optional:    true,
				Description: "Chart version to inspect. Leave unset for the newest one.",
			},
			"content": schema.StringAttribute{
				Computed:    true,
				Description: "The requested document, as returned by Portainer: the repository index when `chart` is unset, otherwise the chart's values, readme or metadata.",
			},
		},
	}
}

func (d *helmChartDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *helmChartDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m helmChartDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if m.Command.IsNull() {
		m.Command = types.StringValue("values")
	}

	repo := m.Repo.ValueString()
	chart := m.Chart.ValueString()

	q := url.Values{}
	q.Set("repo", repo)

	var target, id string
	if chart == "" {
		target = d.client.Endpoint + "/templates/helm?" + q.Encode()
		id = "helm-index-" + repo
	} else {
		command := m.Command.ValueString()
		q.Set("chart", chart)
		if v := m.Version.ValueString(); v != "" {
			q.Set("version", v)
		}
		target = fmt.Sprintf("%s/templates/helm/%s?%s", d.client.Endpoint, command, q.Encode())
		id = fmt.Sprintf("helm-%s-%s-%s", command, repo, chart)
	}

	// Both endpoints answer with a document (YAML or Markdown), not JSON, so
	// this goes through the raw byte helper.
	body, status, err := apiGETWithCodeCtx(ctx, target, d.client.APIKey, d.client)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read from the Helm repository", fmt.Errorf("failed to read from the Helm repository %s: %w", repo, err).Error())
		return
	}
	if status >= http.StatusBadRequest {
		resp.Diagnostics.AddError("Failed to read from the Helm repository", fmt.Sprintf("failed to read from the Helm repository %s: status %d: %s", repo, status, strings.TrimSpace(string(body))))
		return
	}

	m.Content = types.StringValue(string(body))
	m.ID = types.StringValue(id)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
