package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*helmUserRepositoryResource)(nil)
	_ resource.ResourceWithConfigure   = (*helmUserRepositoryResource)(nil)
	_ resource.ResourceWithImportState = (*helmUserRepositoryResource)(nil)
)

func newHelmUserRepositoryResource() resource.Resource { return &helmUserRepositoryResource{} }

type helmUserRepositoryResource struct{ client *APIClient }

type helmUserRepositoryResourceModel struct {
	ID     types.String `tfsdk:"id"`
	UserID types.Int64  `tfsdk:"user_id"`
	URL    types.String `tfsdk:"url"`
}

func (r *helmUserRepositoryResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_helm_user_repository"
}

func (r *helmUserRepositoryResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the Helm repository.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"user_id": schema.Int64Attribute{
				Required:      true,
				Description:   "User identifier.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"url": schema.StringAttribute{
				Required:      true,
				Description:   "Helm repository URL (e.g. https://charts.bitnami.com/bitnami).",
				Validators:    []validator.String{httpURLValidator{}},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
	}
}

func (r *helmUserRepositoryResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *helmUserRepositoryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan helmUserRepositoryResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := map[string]string{"url": plan.URL.ValueString()}
	var result struct {
		ID     int    `json:"Id"`
		URL    string `json:"URL"`
		UserID int    `json:"UserId"`
	}
	p := fmt.Sprintf("/users/%d/helm/repositories", plan.UserID.ValueInt64())
	if err := doJSON(ctx, r.client, http.MethodPost, r.client.Endpoint+p, payload, &result); err != nil {
		resp.Diagnostics.AddError("Failed to create helm user repository", fmt.Errorf("failed to create helm user repository: %w", err).Error())
		return
	}

	// The configured URL is kept: the framework refuses a created object
	// that differs from the configuration, and the next refresh reads
	// Portainer's form of it anyway.
	plan.ID = types.StringValue(strconv.Itoa(result.ID))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *helmUserRepositoryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state helmUserRepositoryResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	p := fmt.Sprintf("/users/%d/helm/repositories", state.UserID.ValueInt64())
	var result struct {
		UserRepositories []struct {
			ID     int    `json:"Id"`
			URL    string `json:"URL"`
			UserID int    `json:"UserId"`
		} `json:"UserRepositories"`
	}
	if err := doJSON(ctx, r.client, http.MethodGet, r.client.Endpoint+p, nil, &result); err != nil {
		resp.Diagnostics.AddError("Failed to read helm user repositories", fmt.Errorf("failed to read helm user repositories: %w", err).Error())
		return
	}

	for _, repo := range result.UserRepositories {
		if strconv.Itoa(repo.ID) == state.ID.ValueString() {
			state.URL = types.StringValue(repo.URL)
			state.UserID = types.Int64Value(int64(repo.UserID))
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}

	// Repository not found - remove from state
	resp.State.RemoveResource(ctx)
}

// Update is never called: every argument requires replacement.
func (r *helmUserRepositoryResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Helm user repositories cannot be updated", "Every argument of portainer_helm_user_repository forces a new resource.")
}

func (r *helmUserRepositoryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state helmUserRepositoryResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p := fmt.Sprintf("/users/%d/helm/repositories/%s", state.UserID.ValueInt64(), state.ID.ValueString())
	if err := doJSON(ctx, r.client, http.MethodDelete, r.client.Endpoint+p, nil, nil); err != nil {
		resp.Diagnostics.AddError("Failed to delete helm user repository", fmt.Errorf("failed to delete helm user repository: %w", err).Error())
	}
}

// ImportState takes the repository ID alone, as the SDKv2 importer did; the
// user_id is then unknown to Read (zero), exactly as before.
func (r *helmUserRepositoryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
