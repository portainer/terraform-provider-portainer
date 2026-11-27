package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource              = (*endpointAssociationResource)(nil)
	_ resource.ResourceWithConfigure = (*endpointAssociationResource)(nil)
)

func newEndpointAssociationResource() resource.Resource { return &endpointAssociationResource{} }

type endpointAssociationResource struct{ client *APIClient }

type endpointAssociationModel struct {
	ID         types.String `tfsdk:"id"`
	EndpointID types.Int64  `tfsdk:"endpoint_id"`
}

func (r *endpointAssociationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_endpoint_association"
}

func (r *endpointAssociationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the de-associated environment.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"endpoint_id": schema.Int64Attribute{
				Required:      true,
				Description:   "Environment (Endpoint) identifier to de-associate",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
		},
	}
}

func (r *endpointAssociationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *endpointAssociationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan endpointAssociationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	endpointAssociationRun(ctx, r.client, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// endpointAssociationRun de-associates the environment; it is the operation
// of both the resource and the portainer_endpoint_association action.
func endpointAssociationRun(ctx context.Context, client *APIClient, plan *endpointAssociationModel, diags *diag.Diagnostics) {
	endpointID := plan.EndpointID.ValueInt64()

	url := fmt.Sprintf("%s/endpoints/%d/association", client.Endpoint, endpointID)
	if err := doJSON(ctx, client, http.MethodPut, url, nil, nil); err != nil {
		diags.AddError("Failed to de-associate endpoint", fmt.Sprintf("failed to de-associate endpoint %d: %s", endpointID, err))
		return
	}
	plan.ID = types.StringValue(strconv.FormatInt(endpointID, 10))
}

func (r *endpointAssociationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state endpointAssociationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	endpointID := state.ID.ValueString()

	// The environment still exists: keep the resource.
	url := fmt.Sprintf("%s/endpoints/%s", r.client.Endpoint, endpointID)
	if err := doJSON(ctx, r.client, http.MethodGet, url, nil, nil); err != nil {
		if isAPINotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read endpoint", fmt.Sprintf("failed to read endpoint %s: %s", endpointID, err))
		return
	}
	id, _ := strconv.ParseInt(endpointID, 10, 64)
	state.EndpointID = types.Int64Value(id)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is never called: every attribute requires replacement.
func (r *endpointAssociationResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Endpoint associations cannot be updated", "Every attribute of portainer_endpoint_association forces a new resource.")
}

// Delete only forgets the resource: Portainer has no re-association API.
func (r *endpointAssociationResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}
