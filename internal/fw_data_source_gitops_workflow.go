package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*gitopsWorkflowDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*gitopsWorkflowDataSource)(nil)
)

func newGitopsWorkflowDataSource() datasource.DataSource { return &gitopsWorkflowDataSource{} }

type gitopsWorkflowDataSource struct{ client *APIClient }

type gitopsWorkflowDSModel struct {
	ID             types.String `tfsdk:"id"`
	WorkflowID     types.Int64  `tfsdk:"workflow_id"`
	Name           types.String `tfsdk:"name"`
	SourceStatus   types.String `tfsdk:"source_status"`
	SourceError    types.String `tfsdk:"source_error"`
	ArtifactStatus types.String `tfsdk:"artifact_status"`
	ArtifactError  types.String `tfsdk:"artifact_error"`
	TargetStatus   types.String `tfsdk:"target_status"`
	TargetError    types.String `tfsdk:"target_error"`
	Healthy        types.Bool   `tfsdk:"healthy"`
	CreationDate   types.Int64  `tfsdk:"creation_date"`
	LastSyncDate   types.Int64  `tfsdk:"last_sync_date"`
}

func (d *gitopsWorkflowDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gitops_workflow"
}

// The schema is kept in step with the elements of portainer_gitops_workflows
// (gitopsWorkflowAttrTypes) by TestGitopsWorkflowDataSource_SchemaMatchesList.
func (d *gitopsWorkflowDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"workflow_id": schema.Int64Attribute{
				Required:    true,
				Description: "Identifier of the GitOps workflow to look up.",
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Description: "Name of the workflow.",
			},
			"source_status": schema.StringAttribute{
				Computed:    true,
				Description: "Status of the source phase: `healthy`, `syncing`, `error`, `paused`, `unknown` or, on the `reconcile` engine, `drifted`.",
			},
			"source_error": schema.StringAttribute{
				Computed:    true,
				Description: "Error reported by the source phase, empty while healthy.",
			},
			"artifact_status": schema.StringAttribute{
				Computed:    true,
				Description: "Status of the artifact phase, using the same values as `source_status`.",
			},
			"artifact_error": schema.StringAttribute{
				Computed:    true,
				Description: "Error reported by the artifact phase, empty while healthy.",
			},
			"target_status": schema.StringAttribute{
				Computed:    true,
				Description: "Status of the target (deployment) phase, using the same values as `source_status`.",
			},
			"target_error": schema.StringAttribute{
				Computed:    true,
				Description: "Error reported by the target phase, empty while healthy.",
			},
			"healthy": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether all three phases report `healthy`.",
			},
			"creation_date": schema.Int64Attribute{
				Computed:    true,
				Description: "Unix timestamp at which the workflow was created.",
			},
			"last_sync_date": schema.Int64Attribute{
				Computed:    true,
				Description: "Unix timestamp of the workflow's last synchronisation.",
			},
		},
	}
}

func (d *gitopsWorkflowDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *gitopsWorkflowDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data gitopsWorkflowDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	client := d.client
	id := int(data.WorkflowID.ValueInt64())

	var workflow gitopsWorkflow
	url := fmt.Sprintf("%s/gitops/workflows/%d", client.Endpoint, id)
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &workflow); err != nil {
		gitopsDSAddError(&resp.Diagnostics, fmt.Errorf("failed to read GitOps workflow %d: %w", id, err))
		return
	}

	// id is carried by the data source ID, not as an attribute of its own.
	v := workflow.attrValues()
	data.Name = v["name"].(types.String)
	data.SourceStatus = v["source_status"].(types.String)
	data.SourceError = v["source_error"].(types.String)
	data.ArtifactStatus = v["artifact_status"].(types.String)
	data.ArtifactError = v["artifact_error"].(types.String)
	data.TargetStatus = v["target_status"].(types.String)
	data.TargetError = v["target_error"].(types.String)
	data.Healthy = v["healthy"].(types.Bool)
	data.CreationDate = v["creation_date"].(types.Int64)
	data.LastSyncDate = v["last_sync_date"].(types.Int64)
	data.ID = types.StringValue(strconv.Itoa(id))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
