package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource              = (*kubernetesNamespaceAccessResource)(nil)
	_ resource.ResourceWithConfigure = (*kubernetesNamespaceAccessResource)(nil)
)

func newKubernetesNamespaceAccessResource() resource.Resource {
	return &kubernetesNamespaceAccessResource{}
}

type kubernetesNamespaceAccessResource struct{ k8sfwBase }

type kubernetesNamespaceAccessModel struct {
	ID            types.String `tfsdk:"id"`
	EndpointID    types.Int64  `tfsdk:"endpoint_id"`
	NamespaceID   types.String `tfsdk:"namespace_id"`
	UsersToAdd    types.List   `tfsdk:"users_to_add"`
	UsersToRemove types.List   `tfsdk:"users_to_remove"`
	TeamsToAdd    types.List   `tfsdk:"teams_to_add"`
	TeamsToRemove types.List   `tfsdk:"teams_to_remove"`
}

func (r *kubernetesNamespaceAccessResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_namespace_access"
}

func (r *kubernetesNamespaceAccessResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the namespace access, `<endpoint_id>/<namespace>`.",
			},
			"endpoint_id": schema.Int64Attribute{
				Required:    true,
				Description: "Identifier of the Portainer Kubernetes endpoint owning the namespace.",
			},
			"namespace_id": schema.StringAttribute{
				Required:    true,
				Description: "Name (identifier) of the Kubernetes namespace whose access policy is being updated.",
			},
			"users_to_add": schema.ListAttribute{
				ElementType: types.Int64Type,
				Optional:    true,
				Description: "List of Portainer user identifiers to grant access to the namespace.",
			},
			"users_to_remove": schema.ListAttribute{
				ElementType: types.Int64Type,
				Optional:    true,
				Description: "List of Portainer user identifiers whose namespace access should be revoked.",
			},
			"teams_to_add": schema.ListAttribute{
				ElementType: types.Int64Type,
				Optional:    true,
				Description: "List of Portainer team identifiers to grant access to the namespace.",
			},
			"teams_to_remove": schema.ListAttribute{
				ElementType: types.Int64Type,
				Optional:    true,
				Description: "List of Portainer team identifiers whose namespace access should be revoked.",
			},
		},
	}
}

// getNamespaceRPN resolves namespace name against the Portainer API.
// Even though API objects have an Id, the /pools/{rpn}/access endpoint
// only works with the namespace name, so we must return Name.
func getNamespaceRPN(ctx context.Context, client *APIClient, environmentID int, namespaceName string) (string, error) {
	url := fmt.Sprintf("%s/kubernetes/%d/namespaces", client.Endpoint, environmentID)

	var namespaces []struct {
		Name string `json:"Name"`
		ID   string `json:"Id"`
	}
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &namespaces); err != nil {
		return "", fmt.Errorf("failed to list namespaces: %w", err)
	}

	for _, ns := range namespaces {
		if ns.Name == namespaceName {
			return ns.Name, nil
		}
	}

	return "", fmt.Errorf("namespace %s not found", namespaceName)
}

func containsColon(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			return true
		}
	}
	return false
}

// namespaceAccessIDs reads a list of identifiers, an empty list (sent as [])
// when unset.
func namespaceAccessIDs(ctx context.Context, l types.List, diags *diag.Diagnostics) []int {
	ids := int64ListToInts(ctx, l, diags)
	if ids == nil {
		ids = []int{}
	}
	return ids
}

func (r *kubernetesNamespaceAccessResource) apply(ctx context.Context, plan *kubernetesNamespaceAccessModel, diags *diag.Diagnostics) {
	client := r.client
	endpointID := int(plan.EndpointID.ValueInt64())
	namespaceID := plan.NamespaceID.ValueString()

	if !containsColon(namespaceID) {
		nsName, err := getNamespaceRPN(ctx, client, endpointID, namespaceID)
		if err != nil {
			diags.AddError("Failed to resolve the namespace", err.Error())
			return
		}
		namespaceID = nsName
	}

	body := map[string]interface{}{
		"usersToAdd":    namespaceAccessIDs(ctx, plan.UsersToAdd, diags),
		"usersToRemove": namespaceAccessIDs(ctx, plan.UsersToRemove, diags),
		"teamsToAdd":    namespaceAccessIDs(ctx, plan.TeamsToAdd, diags),
		"teamsToRemove": namespaceAccessIDs(ctx, plan.TeamsToRemove, diags),
	}
	if diags.HasError() {
		return
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		diags.AddError("Failed to update namespace access", err.Error())
		return
	}

	url := fmt.Sprintf("%s/endpoints/%d/pools/%s/access", client.Endpoint, endpointID, namespaceID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		diags.AddError("Failed to update namespace access", err.Error())
		return
	}

	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		diags.AddError("Failed to update namespace access", err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		data, _ := io.ReadAll(resp.Body)
		diags.AddError("Failed to update namespace access", fmt.Sprintf("failed to update namespace access: %s", string(data)))
		return
	}

	plan.ID = types.StringValue(fmt.Sprintf("%d/%s", endpointID, namespaceID))
}

func (r *kubernetesNamespaceAccessResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan kubernetesNamespaceAccessModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.apply(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read has nothing to refresh: Portainer reports the resulting access, not
// the changes this resource asked for.
func (r *kubernetesNamespaceAccessResource) Read(context.Context, resource.ReadRequest, *resource.ReadResponse) {
}

func (r *kubernetesNamespaceAccessResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan kubernetesNamespaceAccessModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.apply(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete only forgets the change: the access granted or revoked stays.
func (r *kubernetesNamespaceAccessResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}
