package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*endpointGroupResource)(nil)
	_ resource.ResourceWithConfigure   = (*endpointGroupResource)(nil)
	_ resource.ResourceWithImportState = (*endpointGroupResource)(nil)
	_ resource.ResourceWithIdentity    = (*endpointGroupResource)(nil)
)

func newEndpointGroupResource() resource.Resource { return &endpointGroupResource{} }

type endpointGroupResource struct {
	fwIDIdentity
	client *APIClient
}

type endpointGroupResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	TagIDs      types.List   `tfsdk:"tag_ids"`
}

func (r *endpointGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_endpoint_group"
}

func (r *endpointGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the endpoint group.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the Portainer endpoint group. Must be unique within the Portainer instance.",
				Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"description": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Human-readable description of the endpoint group displayed in the Portainer UI.",
				PlanModifiers: []planmodifier.String{egKeepZeroString{}},
			},
			"tag_ids": schema.ListAttribute{
				Optional:    true,
				ElementType: types.Int64Type,
				Description: "List of Portainer tag identifiers associated with this endpoint group.",
			},
		},
	}
}

func (r *endpointGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *endpointGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer func() { fwSetIDIdentity(ctx, resp.State, resp.Identity, &resp.Diagnostics) }()
	var plan endpointGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := plan.Name.ValueString()

	// A group with the same name is adopted and updated instead of duplicated.
	existingID, err := findExistingEndpointGroupByName(ctx, r.client, name)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create endpoint group", fmt.Sprintf("failed to check for existing endpoint group: %s", err))
		return
	}
	if existingID != 0 {
		plan.ID = types.StringValue(strconv.Itoa(existingID))
		r.update(ctx, &plan, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}

	body := struct {
		Name                string  `json:"name"`
		Description         string  `json:"description,omitempty"`
		AssociatedEndpoints []int64 `json:"associatedEndpoints"`
		TagIDs              []int64 `json:"tagIDs"`
	}{Name: name, Description: plan.Description.ValueString()}
	if tagIDs := egInt64Slice(plan.TagIDs); len(tagIDs) > 0 {
		body.TagIDs = tagIDs
	}

	var created portainerEndpointGroup
	if err := doJSON(ctx, r.client, http.MethodPost, r.client.Endpoint+"/endpoint_groups", body, &created); err != nil {
		resp.Diagnostics.AddError("Failed to create endpoint group", fmt.Sprintf("failed to create endpoint group: %s", err))
		return
	}
	plan.ID = types.StringValue(strconv.FormatInt(created.ID, 10))

	// The SDKv2 version re-read the group here; the planned values are kept
	// (the framework requires it) and any drift shows on the next refresh.
	check := plan
	r.readInto(ctx, &check, &resp.Diagnostics, true)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// readInto refreshes m from GET /endpoint_groups/{id}. It reports whether the
// group exists; after a create or an update a missing group is an error.
func (r *endpointGroupResource) readInto(ctx context.Context, m *endpointGroupResourceModel, diags *diag.Diagnostics, mustExist bool) bool {
	id, _ := strconv.ParseInt(m.ID.ValueString(), 10, 64)
	var group portainerEndpointGroup
	if err := doJSON(ctx, r.client, http.MethodGet, fmt.Sprintf("%s/endpoint_groups/%d", r.client.Endpoint, id), nil, &group); err != nil {
		if isAPINotFound(err) && !mustExist {
			return false
		}
		diags.AddError("Failed to read endpoint group", fmt.Sprintf("failed to read endpoint group: %s", err))
		return false
	}
	m.Name = types.StringValue(group.Name)
	m.Description = egOptString(group.Description, m.Description)
	m.TagIDs = egOptInt64List(group.TagIds, m.TagIDs)
	return true
}

func (r *endpointGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	fwSetIDIdentity(ctx, req.State, resp.Identity, &resp.Diagnostics)
	var state endpointGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.readInto(ctx, &state, &resp.Diagnostics, false) {
		if !resp.Diagnostics.HasError() {
			resp.State.RemoveResource(ctx)
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *endpointGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	defer func() { fwSetIDIdentity(ctx, resp.State, resp.Identity, &resp.Diagnostics) }()
	var plan endpointGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.update(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// update sends PUT /endpoint_groups/{id} and re-reads the group, as the
// SDKv2 version did.
func (r *endpointGroupResource) update(ctx context.Context, m *endpointGroupResourceModel, diags *diag.Diagnostics) {
	id, _ := strconv.ParseInt(m.ID.ValueString(), 10, 64)
	body := struct {
		Name        string  `json:"name,omitempty"`
		Description string  `json:"description,omitempty"`
		TagIDs      []int64 `json:"tagIDs"`
	}{Name: m.Name.ValueString(), Description: m.Description.ValueString()}
	if tagIDs := egInt64Slice(m.TagIDs); len(tagIDs) > 0 {
		body.TagIDs = tagIDs
	}
	if err := doJSON(ctx, r.client, http.MethodPut, fmt.Sprintf("%s/endpoint_groups/%d", r.client.Endpoint, id), body, nil); err != nil {
		diags.AddError("Failed to update endpoint group", fmt.Sprintf("failed to update endpoint group: %s", err))
		return
	}
	check := *m
	r.readInto(ctx, &check, diags, true)
}

func (r *endpointGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state endpointGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, _ := strconv.ParseInt(state.ID.ValueString(), 10, 64)
	if err := doJSON(ctx, r.client, http.MethodDelete, fmt.Sprintf("%s/endpoint_groups/%d", r.client.Endpoint, id), nil, nil); err != nil && !isAPINotFound(err) {
		resp.Diagnostics.AddError("Failed to delete endpoint group", fmt.Sprintf("failed to delete endpoint group: %s", err))
	}
}

func (r *endpointGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	fwImportIDOrIdentity(ctx, req, resp)
}

type portainerEndpointGroup struct {
	ID          int64   `json:"Id"`
	Name        string  `json:"Name"`
	Description string  `json:"Description"`
	TagIds      []int64 `json:"TagIds"`
}

// listEndpointGroups returns every environment group (GET /endpoint_groups).
func listEndpointGroups(ctx context.Context, client *APIClient) ([]portainerEndpointGroup, error) {
	var groups []portainerEndpointGroup
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/endpoint_groups", nil, &groups); err != nil {
		return nil, fmt.Errorf("failed to list endpoint groups: %w", err)
	}
	return groups, nil
}

// findExistingEndpointGroupByName returns the ID of the group with that name,
// 0 when there is none.
func findExistingEndpointGroupByName(ctx context.Context, client *APIClient, name string) (int, error) {
	groups, err := listEndpointGroups(ctx, client)
	if err != nil {
		return 0, err
	}
	for _, g := range groups {
		if g.Name == name {
			return int(g.ID), nil
		}
	}
	return 0, nil
}
