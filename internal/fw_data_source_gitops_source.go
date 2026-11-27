package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*gitopsSourceDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*gitopsSourceDataSource)(nil)
)

func newGitopsSourceDataSource() datasource.DataSource { return &gitopsSourceDataSource{} }

type gitopsSourceDataSource struct{ client *APIClient }

type gitopsSourceDSModel struct {
	ID            types.String `tfsdk:"id"`
	SourceID      types.Int64  `tfsdk:"source_id"`
	Name          types.String `tfsdk:"name"`
	URL           types.String `tfsdk:"url"`
	Type          types.String `tfsdk:"type"`
	Status        types.String `tfsdk:"status"`
	StatusError   types.String `tfsdk:"status_error"`
	Interval      types.String `tfsdk:"interval"`
	LastSync      types.Int64  `tfsdk:"last_sync"`
	Username      types.String `tfsdk:"username"`
	TLSSkipVerify types.Bool   `tfsdk:"tls_skip_verify"`
	Public        types.Bool   `tfsdk:"public"`
	UserAccesses  types.List   `tfsdk:"user_accesses"`
	TeamAccesses  types.List   `tfsdk:"team_accesses"`
	Workflows     types.List   `tfsdk:"workflows"`
}

var gitopsSourceDSWorkflowAttrTypes = map[string]attr.Type{
	"id": types.Int64Type, "name": types.StringType, "type": types.StringType,
	"platform": types.StringType, "namespace": types.StringType, "endpoint_id": types.Int64Type,
	"edge_group_ids": types.ListType{ElemType: types.Int64Type},
	"creation_date":  types.Int64Type, "last_sync_date": types.Int64Type,
}

func (d *gitopsSourceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gitops_source"
}

func (d *gitopsSourceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"source_id": schema.Int64Attribute{
				Required:    true,
				Description: "Identifier of the GitOps source to look up.",
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Description: "Name of the source.",
			},
			"url": schema.StringAttribute{
				Computed:    true,
				Description: "Repository URL the source syncs from.",
			},
			"type": schema.StringAttribute{
				Computed:    true,
				Description: "Source type: `git`, `helm` or `oci`.",
			},
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "Sync status: `healthy`, `syncing`, `error`, `paused` or `unknown`.",
			},
			"status_error": schema.StringAttribute{
				Computed:    true,
				Description: "Error reported by the last synchronisation attempt, empty while the source is healthy.",
			},
			"interval": schema.StringAttribute{
				Computed:    true,
				Description: "Polling interval of the source, as a duration string.",
			},
			"last_sync": schema.Int64Attribute{
				Computed:    true,
				Description: "Unix timestamp of the last successful synchronisation.",
			},
			"username": schema.StringAttribute{
				Computed:    true,
				Description: "Username the source authenticates with, empty for an anonymous repository. Portainer never returns the password.",
			},
			"tls_skip_verify": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether TLS certificate verification is skipped when contacting the repository.",
			},
			"public": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether every user can use this source.",
			},
			"user_accesses": schema.ListAttribute{
				Computed:    true,
				ElementType: types.Int64Type,
				Description: "IDs of users granted access to the source.",
			},
			"team_accesses": schema.ListAttribute{
				Computed:    true,
				ElementType: types.Int64Type,
				Description: "IDs of teams granted access to the source.",
			},
			"workflows": schema.ListAttribute{
				Computed:    true,
				ElementType: types.ObjectType{AttrTypes: gitopsSourceDSWorkflowAttrTypes},
				Description: "Workflows deployed from this source. Deleting a source that is still used by a workflow is refused by Portainer.",
			},
		},
	}
}

func (d *gitopsSourceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *gitopsSourceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data gitopsSourceDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	client := d.client
	id := int(data.SourceID.ValueInt64())

	var source gitopsDSSourceDetail
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/gitops/sources/%d", client.Endpoint, id), nil, &source); err != nil {
		gitopsDSAddError(&resp.Diagnostics, fmt.Errorf("failed to read GitOps source %d: %w", id, err))
		return
	}

	var wfList []struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Type     string `json:"type"`
		Platform string `json:"platform"`
		Target   struct {
			Namespace    string `json:"namespace"`
			EndpointID   int    `json:"endpointId"`
			EdgeGroupIDs []int  `json:"edgeGroupIds"`
		} `json:"target"`
		CreationDate int `json:"creationDate"`
		LastSyncDate int `json:"lastSyncDate"`
	}
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/gitops/sources/%d/workflows", client.Endpoint, id), nil, &wfList); err != nil {
		gitopsDSAddError(&resp.Diagnostics, fmt.Errorf("failed to list the workflows of GitOps source %d: %w", id, err))
		return
	}

	elemType := types.ObjectType{AttrTypes: gitopsSourceDSWorkflowAttrTypes}
	workflows := make([]attr.Value, len(wfList))
	for i, w := range wfList {
		workflows[i] = types.ObjectValueMust(gitopsSourceDSWorkflowAttrTypes, map[string]attr.Value{
			"id": types.Int64Value(int64(w.ID)), "name": types.StringValue(w.Name),
			"type": types.StringValue(w.Type), "platform": types.StringValue(w.Platform),
			"namespace":      types.StringValue(w.Target.Namespace),
			"endpoint_id":    types.Int64Value(int64(w.Target.EndpointID)),
			"edge_group_ids": int64ListValue(w.Target.EdgeGroupIDs),
			"creation_date":  types.Int64Value(int64(w.CreationDate)),
			"last_sync_date": types.Int64Value(int64(w.LastSyncDate)),
		})
	}

	data.Name = types.StringValue(source.Name)
	data.URL = types.StringValue(source.URL)
	data.Type = types.StringValue(source.Type)
	data.Status = types.StringValue(source.Status)
	data.StatusError = types.StringValue(source.Error)
	data.Interval = types.StringValue(source.Interval)
	data.LastSync = types.Int64Value(int64(source.LastSync))
	data.Username = types.StringValue(source.Connection.Authentication.Username)
	data.TLSSkipVerify = types.BoolValue(source.Connection.TLSSkipVerify)
	data.Public = types.BoolValue(source.Access.Public)
	data.TeamAccesses = int64ListValue(source.Access.Teams)
	data.UserAccesses = int64ListValue(source.Access.Users)
	data.Workflows = types.ListValueMust(elemType, workflows)
	data.ID = types.StringValue(strconv.Itoa(id))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
