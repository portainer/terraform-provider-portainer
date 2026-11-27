package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Portainer stores the definition of an edge stack and an edge job as a
// file, each behind its own endpoint returning a single JSON string field
// under a different name. edgeStoredFileDataSource is the shared part.
type edgeStoredFileDataSource struct {
	edgeDataSourceBase
	typeName, idAttr, idDesc, what, urlPath, jsonField string
}

var _ datasource.DataSourceWithConfigure = (*edgeStoredFileDataSource)(nil)

func newEdgeStackFileDataSource() datasource.DataSource {
	return &edgeStoredFileDataSource{
		typeName: "_edge_stack_file", idAttr: "edge_stack_id", idDesc: "Identifier of the edge stack whose stack file is read.",
		what: "edge stack", urlPath: "/edge_stacks/%d/file", jsonField: "StackFileContent",
	}
}

func newEdgeJobFileDataSource() datasource.DataSource {
	return &edgeStoredFileDataSource{
		typeName: "_edge_job_file", idAttr: "edge_job_id", idDesc: "Identifier of the edge job whose script is read.",
		what: "edge job", urlPath: "/edge_jobs/%d/file", jsonField: "FileContent",
	}
}

func (d *edgeStoredFileDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + d.typeName
}

func (d *edgeStoredFileDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":     schema.StringAttribute{Computed: true, Description: "`<" + d.idAttr + ">/file`."},
			d.idAttr: schema.Int64Attribute{Required: true, Description: d.idDesc},
			"file_content": schema.StringAttribute{
				Computed:    true,
				Description: "Content of the stored file, exactly as Portainer holds it.",
			},
		},
	}
}

func (d *edgeStoredFileDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var id types.Int64
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(d.idAttr), &id)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var body map[string]string
	url := fmt.Sprintf("%s"+d.urlPath, d.client.Endpoint, id.ValueInt64())
	if err := doJSON(ctx, d.client, http.MethodGet, url, nil, &body); err != nil {
		resp.Diagnostics.AddError("Failed to read the file of the "+d.what, fmt.Sprintf("failed to read the file of %s %d: %s", d.what, id.ValueInt64(), err))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(d.idAttr), id)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("file_content"), types.StringValue(body[d.jsonField]))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(fmt.Sprintf("%d/file", id.ValueInt64())))...)
}

// --- portainer_edge_configuration_files ---

var _ datasource.DataSourceWithConfigure = (*edgeConfigurationFilesDataSource)(nil)

func newEdgeConfigurationFilesDataSource() datasource.DataSource {
	return &edgeConfigurationFilesDataSource{}
}

type edgeConfigurationFilesDataSource struct{ edgeDataSourceBase }

type edgeConfigurationFilesModel struct {
	ID                  types.String `tfsdk:"id"`
	EdgeConfigurationID types.Int64  `tfsdk:"edge_configuration_id"`
	Files               types.String `tfsdk:"files"`
}

func (d *edgeConfigurationFilesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_edge_configuration_files"
}

func (d *edgeConfigurationFilesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "`portainer-edge-configuration-files-<edge_configuration_id>`."},
			"edge_configuration_id": schema.Int64Attribute{
				Required:    true,
				Description: "Identifier of the edge configuration whose files are read.",
			},
			// The endpoint hands back the payload itself rather than a JSON
			// document, so it is carried through unchanged.
			"files": schema.StringAttribute{
				Computed:    true,
				Description: "The configuration's files as Portainer returns them. This is the payload itself, not a JSON document, so it is carried through unchanged.",
			},
		},
	}
}

func (d *edgeConfigurationFilesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m edgeConfigurationFilesModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	configurationID := m.EdgeConfigurationID.ValueInt64()
	body, err := apiGETRaw(ctx, d.client, fmt.Sprintf("%s/edge_configurations/%d/files", d.client.Endpoint, configurationID))
	if err != nil {
		resp.Diagnostics.AddError("Failed to read the edge configuration files", fmt.Sprintf("failed to read the files of edge configuration %d: %s", configurationID, err))
		return
	}
	m.Files = types.StringValue(string(body))
	m.ID = types.StringValue(fmt.Sprintf("portainer-edge-configuration-files-%d", configurationID))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// --- portainer_edge_stack_stagger_status ---

var _ datasource.DataSourceWithConfigure = (*edgeStackStaggerStatusDataSource)(nil)

func newEdgeStackStaggerStatusDataSource() datasource.DataSource {
	return &edgeStackStaggerStatusDataSource{}
}

type edgeStackStaggerStatusDataSource struct{ edgeDataSourceBase }

type edgeStackStaggerStatusModel struct {
	ID          types.String `tfsdk:"id"`
	EdgeStackID types.Int64  `tfsdk:"edge_stack_id"`
	Status      types.String `tfsdk:"status"`
}

func (d *edgeStackStaggerStatusDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_edge_stack_stagger_status"
}

func (d *edgeStackStaggerStatusDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "`portainer-edge-stack-stagger-status-<edge_stack_id>`."},
			"edge_stack_id": schema.Int64Attribute{
				Required:    true,
				Description: "Identifier of the edge stack whose staggered rollout is reported on.",
			},
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "How the staggered rollout is going, as Portainer reports it.",
			},
		},
	}
}

func (d *edgeStackStaggerStatusDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m edgeStackStaggerStatusModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	stackID := m.EdgeStackID.ValueInt64()
	var response struct {
		Status string `json:"status"`
	}
	if err := doJSON(ctx, d.client, http.MethodGet, fmt.Sprintf("%s/edge_stacks/%d/stagger/status", d.client.Endpoint, stackID), nil, &response); err != nil {
		resp.Diagnostics.AddError("Failed to read the stagger status", fmt.Sprintf("failed to read the stagger status of edge stack %d: %s", stackID, err))
		return
	}
	m.Status = types.StringValue(response.Status)
	m.ID = types.StringValue(fmt.Sprintf("portainer-edge-stack-stagger-status-%d", stackID))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
