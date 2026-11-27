package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource              = (*authResource)(nil)
	_ resource.ResourceWithConfigure = (*authResource)(nil)
)

func newAuthResource() resource.Resource { return &authResource{} }

type authResource struct{ client *APIClient }

type authResourceModel struct {
	ID       types.String `tfsdk:"id"`
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
	JWT      types.String `tfsdk:"jwt"`
}

func (r *authResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_auth"
}

func (r *authResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"username": schema.StringAttribute{
				Required:      true,
				Sensitive:     true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "Username used to authenticate against the Portainer API. Stored in state as a sensitive value. Changing this value forces resource recreation.",
			},
			"password": schema.StringAttribute{
				Required:      true,
				Sensitive:     true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "Password used to authenticate against the Portainer API. Stored in state as a sensitive value. Changing this value forces resource recreation.",
			},
			"jwt": schema.StringAttribute{
				Computed:      true,
				Sensitive:     true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Description:   "JWT bearer token issued by Portainer after successful authentication. Computed and stored in state as a sensitive value.",
			},
		},
	}
}

func (r *authResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *authResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan authResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	creds := map[string]string{
		"username": plan.Username.ValueString(),
		"password": plan.Password.ValueString(),
	}
	httpResp, err := r.client.DoRequest(ctx, "POST", "/auth", nil, creds)
	if err != nil {
		resp.Diagnostics.AddError("Failed to authenticate", err.Error())
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(httpResp.Body)
		resp.Diagnostics.AddError("Failed to authenticate", fmt.Sprintf("failed to authenticate: %s", string(data)))
		return
	}

	var response struct {
		JWT string `json:"jwt"`
	}
	if err := json.NewDecoder(httpResp.Body).Decode(&response); err != nil {
		resp.Diagnostics.AddError("Failed to authenticate", err.Error())
		return
	}

	plan.ID = types.StringValue("auth-result")
	plan.JWT = types.StringValue(response.JWT)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read keeps the state: the token is not looked up again.
func (r *authResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {}

// Update is never called: every attribute requires replacement.
func (r *authResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Authentication cannot be updated", "Every attribute of portainer_auth forces a new resource.")
}

func (r *authResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}
