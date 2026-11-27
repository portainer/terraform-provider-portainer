package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource              = (*userMembershipsSyncResource)(nil)
	_ resource.ResourceWithConfigure = (*userMembershipsSyncResource)(nil)
)

// userMembershipsSyncResource refreshes a user's team memberships from the
// configured LDAP/AD or OAuth provider. It is a one-shot action: the
// memberships it produces are read through the team membership resources.
func newUserMembershipsSyncResource() resource.Resource { return &userMembershipsSyncResource{} }

type userMembershipsSyncResource struct{ client *APIClient }

type userMembershipsSyncResourceModel struct {
	ID       types.String `tfsdk:"id"`
	UserID   types.Int64  `tfsdk:"user_id"`
	Triggers types.Map    `tfsdk:"triggers"`
}

func (r *userMembershipsSyncResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_memberships_sync"
}

func (r *userMembershipsSyncResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"user_id": schema.Int64Attribute{
				Required:      true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
				Description:   "Identifier of the user whose team memberships are synchronised.",
			},
			"triggers": schema.MapAttribute{
				Optional:      true,
				ElementType:   types.StringType,
				PlanModifiers: []planmodifier.Map{mapplanmodifier.RequiresReplace()},
				Description:   "Arbitrary values that force another synchronisation when they change. A sync is a one-shot action, so without a trigger the resource runs once and then stays put.",
			},
		},
	}
}

func (r *userMembershipsSyncResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *userMembershipsSyncResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan userMembershipsSyncResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	userMembershipsSyncRun(ctx, r.client, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// userMembershipsSyncRun synchronises the memberships; it is the operation of
// both the resource and the portainer_user_memberships_sync action.
func userMembershipsSyncRun(ctx context.Context, client *APIClient, plan *userMembershipsSyncResourceModel, diags *diag.Diagnostics) {
	userID := plan.UserID.ValueInt64()

	syncURL := fmt.Sprintf("%s/users/%d/memberships/sync", client.Endpoint, userID)
	if err := doJSON(ctx, client, http.MethodPost, syncURL, nil, nil); err != nil {
		diags.AddError("Failed to synchronise team memberships", fmt.Sprintf("failed to synchronise the team memberships of user %d: %s", userID, err))
		return
	}

	plan.ID = types.StringValue(fmt.Sprintf("%d-memberships-sync-%d", userID, makeTimestamp()))
}

// Read keeps the state: the sync leaves nothing to read back.
func (r *userMembershipsSyncResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {
}

// Update is never called: every argument forces a new sync.
func (r *userMembershipsSyncResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Membership syncs cannot be updated", "Every attribute of portainer_user_memberships_sync forces a new resource.")
}

// Delete only forgets the run.
func (r *userMembershipsSyncResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}
