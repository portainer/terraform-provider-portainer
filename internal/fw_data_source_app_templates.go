package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*appTemplatesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*appTemplatesDataSource)(nil)
)

func newAppTemplatesDataSource() datasource.DataSource { return &appTemplatesDataSource{} }

type appTemplatesDataSource struct{ client *APIClient }

type appTemplatesDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	TemplateID  types.Int64  `tfsdk:"template_id"`
	Version     types.String `tfsdk:"version"`
	Templates   types.List   `tfsdk:"templates"`
	FileContent types.String `tfsdk:"file_content"`
}

type appTemplateModel struct {
	ID            types.Int64  `tfsdk:"id"`
	Title         types.String `tfsdk:"title"`
	Description   types.String `tfsdk:"description"`
	Type          types.Int64  `tfsdk:"type"`
	Platform      types.String `tfsdk:"platform"`
	Categories    types.List   `tfsdk:"categories"`
	Image         types.String `tfsdk:"image"`
	RepositoryURL types.String `tfsdk:"repository_url"`
}

var appTemplateAttrTypes = map[string]attr.Type{
	"id":             types.Int64Type,
	"title":          types.StringType,
	"description":    types.StringType,
	"type":           types.Int64Type,
	"platform":       types.StringType,
	"categories":     types.ListType{ElemType: types.StringType},
	"image":          types.StringType,
	"repository_url": types.StringType,
}

func (d *appTemplatesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_templates"
}

func (d *appTemplatesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the data source, built from the catalogue version.",
			},
			"template_id": schema.Int64Attribute{
				Optional:    true,
				Description: "Identifier of a template whose stack file should also be fetched. Leave unset to only list the catalogue.",
			},
			"version": schema.StringAttribute{
				Computed:    true,
				Description: "Version of the template catalogue Portainer is serving.",
			},
			"templates": schema.ListAttribute{
				Computed:    true,
				Description: "Templates in the catalogue Portainer is configured to serve.",
				ElementType: types.ObjectType{AttrTypes: appTemplateAttrTypes},
			},
			"file_content": schema.StringAttribute{
				Computed:    true,
				Description: "Stack file of the template named by `template_id`, empty when that argument is unset.",
			},
		},
	}
}

func (d *appTemplatesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *appTemplatesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m appTemplatesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var list struct {
		Version   string `json:"version"`
		Templates []struct {
			ID          int      `json:"Id"`
			Title       string   `json:"title"`
			Description string   `json:"description"`
			Type        int      `json:"type"`
			Platform    string   `json:"platform"`
			Categories  []string `json:"categories"`
			Image       string   `json:"image"`
			Repository  struct {
				URL string `json:"url"`
			} `json:"repository"`
		} `json:"templates"`
	}
	if err := doJSON(ctx, d.client, http.MethodGet, d.client.Endpoint+"/templates", nil, &list); err != nil {
		resp.Diagnostics.AddError("Failed to list the application templates", fmt.Errorf("failed to list the application templates: %w", err).Error())
		return
	}

	templates := make([]appTemplateModel, len(list.Templates))
	for i, t := range list.Templates {
		categories := t.Categories
		if categories == nil {
			categories = []string{}
		}
		cats, diags := types.ListValueFrom(ctx, types.StringType, categories)
		resp.Diagnostics.Append(diags...)
		templates[i] = appTemplateModel{
			ID: types.Int64Value(int64(t.ID)), Title: types.StringValue(t.Title),
			Description: types.StringValue(t.Description), Type: types.Int64Value(int64(t.Type)),
			Platform: types.StringValue(t.Platform), Categories: cats,
			Image: types.StringValue(t.Image), RepositoryURL: types.StringValue(t.Repository.URL),
		}
	}

	// The file endpoint is a POST even though it only reads — that is how
	// Portainer models it. Like SDKv2's GetOk, template 0 counts as unset.
	fileContent := ""
	if id := m.TemplateID.ValueInt64(); id != 0 {
		var file struct {
			FileContent string `json:"FileContent"`
		}
		fileURL := fmt.Sprintf("%s/templates/%d/file", d.client.Endpoint, id)
		if err := doJSON(ctx, d.client, http.MethodPost, fileURL, nil, &file); err != nil {
			resp.Diagnostics.AddError("Failed to read the template file", fmt.Errorf("failed to read the file of template %d: %w", id, err).Error())
			return
		}
		fileContent = file.FileContent
	}

	tl, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: appTemplateAttrTypes}, templates)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	m.Version = types.StringValue(list.Version)
	m.Templates = tl
	m.FileContent = types.StringValue(fileContent)
	m.ID = types.StringValue("app-templates-" + list.Version)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
