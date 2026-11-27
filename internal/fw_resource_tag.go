package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*tagResource)(nil)
	_ resource.ResourceWithConfigure   = (*tagResource)(nil)
	_ resource.ResourceWithImportState = (*tagResource)(nil)
	_ resource.ResourceWithIdentity    = (*tagResource)(nil)
)

func newTagResource() resource.Resource { return &tagResource{} }

type tagResource struct {
	fwIDIdentity
	client *APIClient
}

type tagResourceModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

func (r *tagResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tag"
}

func (r *tagResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the tag.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:      true,
				Description:   "Name of the tag. Must be unique within the Portainer instance. Changing this value forces resource recreation.",
				Validators:    []validator.String{stringvalidator.LengthAtLeast(1)},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
	}
}

func (r *tagResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *tagResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan tagResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var created portainerTag
	body := map[string]interface{}{"name": plan.Name.ValueString()}
	if err := doJSON(ctx, r.client, http.MethodPost, r.client.Endpoint+"/tags", body, &created); err != nil {
		resp.Diagnostics.AddError("Failed to create tag", err.Error())
		return
	}

	plan.ID = types.StringValue(strconv.FormatInt(created.ID, 10))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	fwSetIDIdentity(ctx, resp.State, resp.Identity, &resp.Diagnostics)
}

func (r *tagResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state tagResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	fwSetIDIdentity(ctx, req.State, resp.Identity, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// Portainer has no GET /tags/{id}, so list and filter.
	tagList, err := listTags(ctx, r.client)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read tag", err.Error())
		return
	}
	id, _ := strconv.ParseInt(state.ID.ValueString(), 10, 64)
	for _, tag := range tagList {
		if tag.ID == id {
			state.Name = types.StringValue(tag.Name)
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}

// Update is never called: every attribute requires replacement.
func (r *tagResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Tags cannot be updated", "Every attribute of portainer_tag forces a new resource.")
}

func (r *tagResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state tagResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, _ := strconv.ParseInt(state.ID.ValueString(), 10, 64)
	if err := doJSON(ctx, r.client, http.MethodDelete, fmt.Sprintf("%s/tags/%d", r.client.Endpoint, id), nil, nil); err != nil && !isAPINotFound(err) {
		resp.Diagnostics.AddError("Failed to delete tag", err.Error())
	}
}

func (r *tagResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	fwImportIDOrIdentity(ctx, req, resp)
}

type portainerTag struct {
	ID   int64  `json:"id"`
	Name string `json:"Name"`
}

// listTags returns every tag (GET /tags).
func listTags(ctx context.Context, client *APIClient) ([]portainerTag, error) {
	var tagList []portainerTag
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/tags", nil, &tagList); err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}
	return tagList, nil
}
