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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*edgeGroupResource)(nil)
	_ resource.ResourceWithConfigure   = (*edgeGroupResource)(nil)
	_ resource.ResourceWithImportState = (*edgeGroupResource)(nil)
	_ resource.ResourceWithIdentity    = (*edgeGroupResource)(nil)
)

func newEdgeGroupResource() resource.Resource { return &edgeGroupResource{} }

type edgeGroupResource struct {
	fwIDIdentity
	client *APIClient
}

type edgeGroupResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Dynamic      types.Bool   `tfsdk:"dynamic"`
	PartialMatch types.Bool   `tfsdk:"partial_match"`
	Endpoints    types.List   `tfsdk:"endpoints"`
	TagIDs       types.List   `tfsdk:"tag_ids"`
}

func (r *edgeGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_edge_group"
}

func (r *edgeGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the Edge group.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
				Description: "Name of the Portainer Edge group.",
			},
			"dynamic": schema.BoolAttribute{
				Required:    true,
				Description: "Whether the Edge group is dynamic (membership determined by tags) or static (explicit endpoints).",
			},
			"partial_match": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "For dynamic groups, whether to match endpoints with any (true) or all (false) of the specified tags.",
			},
			"endpoints": schema.ListAttribute{
				ElementType: types.Int64Type,
				Optional:    true,
				Description: "List of Portainer environment (endpoint) IDs that are statically assigned to the Edge group. Used when dynamic is false.",
			},
			"tag_ids": schema.ListAttribute{
				ElementType: types.Int64Type,
				Optional:    true,
				Description: "List of tag IDs used to dynamically select endpoints. Used when dynamic is true.",
			},
		},
	}
}

func (r *edgeGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func findExistingEdgeGroupByName(ctx context.Context, client *APIClient, name string) (int, error) {
	var groups []map[string]interface{}
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/edge_groups", client.Endpoint), nil, &groups); err != nil {
		return 0, fmt.Errorf("failed to list edge groups: %w", err)
	}

	for _, g := range groups {
		if g["Name"] == name {
			if id, ok := g["Id"].(float64); ok {
				return int(id), nil
			}
		}
	}

	return 0, nil
}

// edgeGroupPayload is the POST/PUT body; endpoints and tag IDs are only sent
// when the configuration lists some.
func edgeGroupPayload(ctx context.Context, m *edgeGroupResourceModel, diags *diag.Diagnostics) map[string]interface{} {
	payload := map[string]interface{}{
		"name":         m.Name.ValueString(),
		"dynamic":      m.Dynamic.ValueBool(),
		"partialMatch": m.PartialMatch.ValueBool(),
	}
	if v := int64ListToInts(ctx, m.Endpoints, diags); len(v) > 0 {
		payload["endpoints"] = v
	}
	if v := int64ListToInts(ctx, m.TagIDs, diags); len(v) > 0 {
		payload["tagIDs"] = v
	}
	return payload
}

func (r *edgeGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer func() { fwSetIDIdentity(ctx, resp.State, resp.Identity, &resp.Diagnostics) }()
	var plan edgeGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	existingID, err := findExistingEdgeGroupByName(ctx, r.client, plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to create edge group", fmt.Sprintf("failed to check for existing edge group: %s", err))
		return
	}
	payload := edgeGroupPayload(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if existingID != 0 {
		// A group of that name is adopted and updated, as the SDKv2 version did.
		plan.ID = types.StringValue(strconv.Itoa(existingID))
		if err := doJSON(ctx, r.client, http.MethodPut, fmt.Sprintf("%s/edge_groups/%d", r.client.Endpoint, existingID), payload, nil); err != nil {
			resp.Diagnostics.AddError("Failed to update edge group", fmt.Sprintf("failed to update edge group: %s", err))
			return
		}
	} else {
		var result struct {
			ID int `json:"Id"`
		}
		if err := doJSON(ctx, r.client, http.MethodPost, fmt.Sprintf("%s/edge_groups", r.client.Endpoint), payload, &result); err != nil {
			resp.Diagnostics.AddError("Failed to create edge group", fmt.Sprintf("failed to create edge group: %s", err))
			return
		}
		plan.ID = types.StringValue(strconv.Itoa(result.ID))
	}

	r.readInto(ctx, &plan, &resp.State, &resp.Diagnostics)
	edgeKeepCreatedOnError(ctx, req.Plan, plan.ID.ValueString(), &resp.State, &resp.Diagnostics)
}

func (r *edgeGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	fwSetIDIdentity(ctx, req.State, resp.Identity, &resp.Diagnostics)
	var state edgeGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.readInto(ctx, &state, &resp.State, &resp.Diagnostics)
}

// readInto reads the group into m and stores it in state, or removes the
// resource when Portainer no longer has it.
func (r *edgeGroupResource) readInto(ctx context.Context, m *edgeGroupResourceModel, state *tfsdk.State, diags *diag.Diagnostics) {
	var group struct {
		Name         string `json:"Name"`
		Dynamic      bool   `json:"Dynamic"`
		PartialMatch bool   `json:"PartialMatch"`
		TagIDs       []int  `json:"TagIds"`
		Endpoints    []int  `json:"Endpoints"`
	}
	if err := doJSON(ctx, r.client, http.MethodGet, fmt.Sprintf("%s/edge_groups/%s", r.client.Endpoint, m.ID.ValueString()), nil, &group); err != nil {
		if isAPINotFound(err) {
			state.RemoveResource(ctx)
			return
		}
		diags.AddError("Failed to read edge group", "failed to read edge group")
		return
	}

	m.Name = types.StringValue(group.Name)
	m.Dynamic = types.BoolValue(group.Dynamic)
	m.PartialMatch = types.BoolValue(group.PartialMatch)
	m.TagIDs = edgeIntListFromAPI(m.TagIDs, group.TagIDs)
	m.Endpoints = edgeIntListFromAPI(m.Endpoints, group.Endpoints)
	diags.Append(state.Set(ctx, m)...)
}

func (r *edgeGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	defer func() { fwSetIDIdentity(ctx, resp.State, resp.Identity, &resp.Diagnostics) }()
	var plan edgeGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	payload := edgeGroupPayload(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := doJSON(ctx, r.client, http.MethodPut, fmt.Sprintf("%s/edge_groups/%s", r.client.Endpoint, plan.ID.ValueString()), payload, nil); err != nil {
		resp.Diagnostics.AddError("Failed to update edge group", fmt.Sprintf("failed to update edge group: %s", err))
		return
	}
	r.readInto(ctx, &plan, &resp.State, &resp.Diagnostics)
}

func (r *edgeGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state edgeGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Portainer's message says why a group cannot go, e.g. an Edge stack
	// still deployed to it; a group already gone is fine.
	if err := doJSON(ctx, r.client, http.MethodDelete, fmt.Sprintf("%s/edge_groups/%s", r.client.Endpoint, state.ID.ValueString()), nil, nil); err != nil && !isAPINotFound(err) {
		resp.Diagnostics.AddError("Failed to delete edge group", fmt.Sprintf("failed to delete edge group: %s", err))
	}
}

func (r *edgeGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	fwImportIDOrIdentity(ctx, req, resp)
}
