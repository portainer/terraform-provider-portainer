package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*customTemplateFileDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*customTemplateFileDataSource)(nil)
)

func newCustomTemplateFileDataSource() datasource.DataSource { return &customTemplateFileDataSource{} }

type customTemplateFileDataSource struct{ client *APIClient }

type customTemplateFileDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	TemplateID  types.Int64  `tfsdk:"template_id"`
	FileContent types.String `tfsdk:"file_content"`
}

func (d *customTemplateFileDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_custom_template_file"
}

func (d *customTemplateFileDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":           schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"template_id":  schema.Int64Attribute{Required: true, Description: "Identifier of the custom template whose stack file is read."},
			"file_content": schema.StringAttribute{Computed: true, Description: "Content of the stored file, exactly as Portainer holds it."},
		},
	}
}

func (d *customTemplateFileDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *customTemplateFileDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m customTemplateFileDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := m.TemplateID.ValueInt64()

	// Portainer returns the stored file as a single JSON string field.
	var body map[string]string
	if err := doJSON(ctx, d.client, http.MethodGet, fmt.Sprintf("%s/custom_templates/%d/file", d.client.Endpoint, id), nil, &body); err != nil {
		resp.Diagnostics.AddError("Failed to read the custom template file", fmt.Sprintf("failed to read the file of custom template %d: %s", id, err))
		return
	}
	m.FileContent = types.StringValue(body["FileContent"])
	m.ID = types.StringValue(fmt.Sprintf("%d/file", id))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
