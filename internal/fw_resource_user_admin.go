package internal

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*userAdminResource)(nil)
	_ resource.ResourceWithConfigure   = (*userAdminResource)(nil)
	_ resource.ResourceWithImportState = (*userAdminResource)(nil)
)

func newUserAdminResource() resource.Resource { return &userAdminResource{} }

type userAdminResource struct{ client *APIClient }

type userAdminResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Username    types.String `tfsdk:"username"`
	Password    types.String `tfsdk:"password"`
	SetupToken  types.String `tfsdk:"setup_token"`
	Initialized types.Bool   `tfsdk:"initialized"`
}

func (r *userAdminResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_admin"
}

func (r *userAdminResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"username": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("admin"),
				Description: "Username of the admin account to initialize (defaults to 'admin').",
			},
			"password": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "Password for the admin account. Only used during initial bootstrap.",
			},
			"setup_token": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				// A bootstrap credential only the init call needs, so it is
				// never kept in state.
				WriteOnly:   true,
				Description: "Write-only value sent as the `X-Setup-Token` header (never stored in Terraform state; requires Terraform 1.11 or later). Portainer 2.45 and later require it to initialize a fresh instance unless started with `--no-setup-token`; the token is printed in the Portainer log (`setup_token=...`) or set with `--setup-token` / `PORTAINER_SETUP_TOKEN`.",
			},
			"initialized": schema.BoolAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
				Description:   "Whether the admin user has been initialized via this resource.",
			},
		},
	}
}

func (r *userAdminResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *userAdminResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan userAdminResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	client := r.client
	plan.SetupToken = types.StringNull()
	plan.Initialized = types.BoolValue(true)

	// Ask first instead of relying on 409 from the init call: Portainer 2.45
	// checks the setup token before it looks for an administrator, so until
	// its next restart an initialized instance answers 403 to a call without
	// the token.
	exists, err := adminExists(ctx, client)
	if err != nil {
		resp.Diagnostics.AddError("Failed to initialize admin user", err.Error())
		return
	}
	if exists {
		plan.ID = types.StringValue("portainer-admin")
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}

	body := struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}{Username: plan.Username.ValueString(), Password: plan.Password.ValueString()}

	headers := map[string]string{}
	if token := writeOnlyConfigString(ctx, req.Config, path.Root("setup_token"), &resp.Diagnostics); token != "" {
		headers["X-Setup-Token"] = token
	}
	if resp.Diagnostics.HasError() {
		return
	}

	var created portainerUser
	if err := doJSONPublic(ctx, client, http.MethodPost, client.Endpoint+"/users/admin/init", body, &created, headers); err != nil {
		// Treat 409 (admin already initialized) as a successful, idempotent create.
		var se *apiStatusError
		if errors.As(err, &se) && se.StatusCode == http.StatusConflict {
			plan.ID = types.StringValue("portainer-admin")
			resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
			return
		}
		resp.Diagnostics.AddError("Failed to initialize admin user", fmt.Sprintf("failed to initialize admin user: %s", err))
		return
	}

	if created.ID != 0 {
		plan.ID = types.StringValue(strconv.FormatInt(created.ID, 10))
	} else {
		plan.ID = types.StringValue("portainer-admin")
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read keeps the resource once the instance has an administrator, so the
// plan settles after the first apply instead of proposing the bootstrap again
// on every run. Only an instance that lost its administrator (a reset or a
// fresh data volume) drops the resource, which plans the initialization
// again.
func (r *userAdminResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userAdminResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	exists, err := adminExists(ctx, r.client)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read admin user", err.Error())
		return
	}
	if !exists {
		resp.State.RemoveResource(ctx)
		return
	}
	state.Initialized = types.BoolValue(true)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update stores the new arguments without calling Portainer: the init API
// runs once, so a changed username or password cannot be applied to an
// initialized instance.
func (r *userAdminResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state userAdminResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID
	plan.SetupToken = types.StringNull()
	if plan.Initialized.IsUnknown() {
		plan.Initialized = state.Initialized
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete only forgets the resource: an administrator cannot be removed.
func (r *userAdminResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

func (r *userAdminResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
