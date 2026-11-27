package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource              = (*sshKeygenResource)(nil)
	_ resource.ResourceWithConfigure = (*sshKeygenResource)(nil)
)

func newSSHKeygenResource() resource.Resource { return &sshKeygenResource{} }

type sshKeygenResource struct{ client *APIClient }

type sshKeygenResourceModel struct {
	ID      types.String `tfsdk:"id"`
	Public  types.String `tfsdk:"public"`
	Private types.String `tfsdk:"private"`
}

func (r *sshKeygenResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sshkeygen"
}

func (r *sshKeygenResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the generated key pair.",
				PlanModifiers: keep,
			},
			"public": schema.StringAttribute{
				Computed:      true,
				Description:   "Generated public key",
				PlanModifiers: keep,
			},
			"private": schema.StringAttribute{
				Computed:      true,
				Sensitive:     true,
				Description:   "Generated private key",
				PlanModifiers: keep,
			},
		},
	}
}

func (r *sshKeygenResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *sshKeygenResource) Create(ctx context.Context, _ resource.CreateRequest, resp *resource.CreateResponse) {
	client := r.client
	const summary = "Failed to generate SSH key"

	// Business Edition 2.45 rejects a request without a body ("Invalid request
	// payload: EOF"); 3.x takes either.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/sshkeygen", client.Endpoint), strings.NewReader("{}"))
	if err != nil {
		resp.Diagnostics.AddError(summary, err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if err := setAuthHeader(req, client); err != nil {
		resp.Diagnostics.AddError(summary, err.Error())
		return
	}

	httpResp, err := client.HTTPClient.Do(req)
	if err != nil {
		resp.Diagnostics.AddError(summary, err.Error())
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= 400 {
		body, _ := io.ReadAll(httpResp.Body)
		resp.Diagnostics.AddError(summary, fmt.Sprintf("failed to generate SSH key: %s", string(body)))
		return
	}

	var keypair struct {
		Public  string `json:"public"`
		Private string `json:"private"`
	}
	if err := json.NewDecoder(httpResp.Body).Decode(&keypair); err != nil {
		resp.Diagnostics.AddError(summary, fmt.Sprintf("failed to decode response: %s", err))
		return
	}

	state := sshKeygenResourceModel{
		ID:      types.StringValue(strconv.Itoa(len(keypair.Public))),
		Public:  types.StringValue(keypair.Public),
		Private: types.StringValue(keypair.Private),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Read is a no-op: the key pair is generated once and never read back.
func (r *sshKeygenResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {
}

// Update is never called: the resource has no arguments.
func (r *sshKeygenResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("SSH key pairs cannot be updated", "portainer_sshkeygen has no arguments to update.")
}

// Delete only removes the key pair from the state.
func (r *sshKeygenResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}
