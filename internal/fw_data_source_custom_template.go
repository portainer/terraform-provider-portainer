package internal

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*customTemplateDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*customTemplateDataSource)(nil)
)

func newCustomTemplateDataSource() datasource.DataSource { return &customTemplateDataSource{} }

type customTemplateDataSource struct{ client *APIClient }

type customTemplateDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Title       types.String `tfsdk:"title"`
	Description types.String `tfsdk:"description"`
	Type        types.Int64  `tfsdk:"type"`
}

func (d *customTemplateDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_custom_template"
}

func (d *customTemplateDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"title":       schema.StringAttribute{Required: true, Description: "Title of the custom template to look up in Portainer."},
			"description": schema.StringAttribute{Computed: true, Description: "Description of the matched custom template."},
			"type":        schema.Int64Attribute{Computed: true, Description: "Template type: 1 = Swarm stack, 2 = Compose stack, 3 = Kubernetes manifest."},
		},
	}
}

func (d *customTemplateDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *customTemplateDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m customTemplateDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	title := m.Title.ValueString()

	templates, err := listCustomTemplates(ctx, d.client)
	if err != nil {
		resp.Diagnostics.AddError("Failed to list custom templates", err.Error())
		return
	}
	for _, t := range templates {
		if t.Title == title {
			m.ID = types.StringValue(strconv.FormatInt(t.ID, 10))
			m.Description = types.StringValue(t.Description)
			m.Type = types.Int64Value(t.Type)
			resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
			return
		}
	}
	resp.Diagnostics.AddError("Custom template not found", fmt.Sprintf("custom template %s not found", title))
}
