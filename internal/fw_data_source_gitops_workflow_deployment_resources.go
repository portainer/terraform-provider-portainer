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
	_ datasource.DataSource              = (*gitopsWorkflowDeploymentResourcesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*gitopsWorkflowDeploymentResourcesDataSource)(nil)
)

// newGitopsWorkflowDeploymentResourcesDataSource lists the Kubernetes
// resources a reconciled deployment declares, read from the manifests
// Portainer materialised for it - one copy of what every targeted environment
// applies, at the revision the deployment last assembled. A Helm chart lists
// nothing: what a release contains is only known once Helm has rendered it.
func newGitopsWorkflowDeploymentResourcesDataSource() datasource.DataSource {
	return &gitopsWorkflowDeploymentResourcesDataSource{}
}

type gitopsWorkflowDeploymentResourcesDataSource struct{ client *APIClient }

type gitopsWorkflowDeploymentResourcesModel struct {
	ID           types.String `tfsdk:"id"`
	WorkflowID   types.Int64  `tfsdk:"workflow_id"`
	DeploymentID types.Int64  `tfsdk:"deployment_id"`
	Resources    types.List   `tfsdk:"resources"`
}

var gitopsDeploymentResourceAttrTypes = map[string]attr.Type{
	"api_version": types.StringType, "kind": types.StringType, "name": types.StringType, "namespace": types.StringType,
}

func (d *gitopsWorkflowDeploymentResourcesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gitops_workflow_deployment_resources"
}

func (d *gitopsWorkflowDeploymentResourcesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"workflow_id": schema.Int64Attribute{
				Required:    true,
				Description: "Identifier of the GitOps workflow.",
			},
			"deployment_id": schema.Int64Attribute{
				Required:    true,
				Description: "Identifier of the reconciled deployment: the `artifact_id` of an artifact of a `portainer_gitops_workflow` on the `reconcile` engine.",
			},
			"resources": schema.ListAttribute{
				Computed:    true,
				ElementType: types.ObjectType{AttrTypes: gitopsDeploymentResourceAttrTypes},
				Description: "Kubernetes resources the deployment declares.",
			},
		},
	}
}

func (d *gitopsWorkflowDeploymentResourcesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *gitopsWorkflowDeploymentResourcesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data gitopsWorkflowDeploymentResourcesModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	client := d.client
	workflowID := int(data.WorkflowID.ValueInt64())
	deploymentID := int(data.DeploymentID.ValueInt64())

	var list []struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
		Name       string `json:"name"`
		Namespace  string `json:"namespace"`
	}
	endpoint := fmt.Sprintf("%s/gitops/workflows/%d/deployments/%d/resources", client.Endpoint, workflowID, deploymentID)
	if err := doJSON(ctx, client, http.MethodGet, endpoint, nil, &list); err != nil {
		err = explainUnsupported(ctx, client, err, "portainer_gitops_workflow_deployment_resources", requiresBE30)
		gitopsDSAddError(&resp.Diagnostics, fmt.Errorf("failed to read the resources of deployment %d of GitOps workflow %d: %w", deploymentID, workflowID, err))
		return
	}

	resources := make([]attr.Value, len(list))
	for i, r := range list {
		resources[i] = types.ObjectValueMust(gitopsDeploymentResourceAttrTypes, map[string]attr.Value{
			"api_version": types.StringValue(r.APIVersion), "kind": types.StringValue(r.Kind),
			"name": types.StringValue(r.Name), "namespace": types.StringValue(r.Namespace),
		})
	}
	data.Resources = types.ListValueMust(types.ObjectType{AttrTypes: gitopsDeploymentResourceAttrTypes}, resources)
	data.ID = types.StringValue(fmt.Sprintf("%d:%d", workflowID, deploymentID))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
