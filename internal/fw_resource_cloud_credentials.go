package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// CloudCredentialPayload is the body of the cloud credential requests.
type CloudCredentialPayload struct {
	Provider    string                 `json:"provider"`
	Name        string                 `json:"name"`
	Credentials map[string]interface{} `json:"credentials"`
}

var (
	_ resource.Resource                = (*cloudCredentialsResource)(nil)
	_ resource.ResourceWithConfigure   = (*cloudCredentialsResource)(nil)
	_ resource.ResourceWithImportState = (*cloudCredentialsResource)(nil)
)

func newCloudCredentialsResource() resource.Resource { return &cloudCredentialsResource{} }

type cloudCredentialsResource struct{ client *APIClient }

type cloudCredentialsModel struct {
	ID            types.String `tfsdk:"id"`
	CloudProvider types.String `tfsdk:"cloud_provider"`
	Name          types.String `tfsdk:"name"`
	Credentials   types.Map    `tfsdk:"credentials"`
}

func (r *cloudCredentialsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_credentials"
}

func (r *cloudCredentialsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"cloud_provider": schema.StringAttribute{
				Required:    true,
				Description: "Cloud provider name (e.g., aws, gcp, digitalocean)",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
				Description: "Human-readable name of the credentials",
			},
			"credentials": schema.MapAttribute{
				Required:    true,
				Sensitive:   true,
				ElementType: types.StringType,
				Description: "JSON-encoded credentials for the provider",
			},
		},
	}
}

func (r *cloudCredentialsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *cloudCredentialsResource) payload(ctx context.Context, m *cloudCredentialsModel, resp interface{ AddError(string, string) }) (CloudCredentialPayload, bool) {
	var creds map[string]string
	if d := m.Credentials.ElementsAs(ctx, &creds, false); d.HasError() {
		resp.AddError("Invalid credentials", fmt.Sprint(d))
		return CloudCredentialPayload{}, false
	}
	out := make(map[string]interface{}, len(creds))
	for k, v := range creds {
		out[k] = v
	}
	return CloudCredentialPayload{
		Provider:    m.CloudProvider.ValueString(),
		Name:        m.Name.ValueString(),
		Credentials: out,
	}, true
}

func (r *cloudCredentialsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan cloudCredentialsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	payload, ok := r.payload(ctx, &plan, &resp.Diagnostics)
	if !ok {
		return
	}
	var result struct {
		ID int `json:"id"`
	}
	if err := doJSON(ctx, r.client, http.MethodPost, r.client.Endpoint+"/cloud/credentials", payload, &result); err != nil {
		resp.Diagnostics.AddError("Failed to create cloud credential", fmt.Sprintf("failed to create cloud credential: %s", err))
		return
	}
	plan.ID = types.StringValue(strconv.Itoa(result.ID))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *cloudCredentialsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state cloudCredentialsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var result struct {
		ID          int                    `json:"id"`
		Name        string                 `json:"name"`
		Provider    string                 `json:"provider"`
		Credentials map[string]interface{} `json:"credentials"`
	}
	if err := doJSON(ctx, r.client, http.MethodGet, fmt.Sprintf("%s/cloud/credentials/%s", r.client.Endpoint, state.ID.ValueString()), nil, &result); err != nil {
		if isAPINotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read cloud credential", fmt.Sprintf("failed to read cloud credential: %s", err))
		return
	}

	prior := stringMapToGo(ctx, state.Credentials, &resp.Diagnostics)
	state.Name = types.StringValue(result.Name)
	state.CloudProvider = types.StringValue(result.Provider)
	state.Credentials = stringMapValue(unmaskCredentials(result.Credentials, prior))
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// unmaskCredentials keeps the configured value of every secret Portainer
// returns masked ("******"): the mask read into state planned a change on
// every run. A key that is no longer there, or a plain value that changed,
// still shows as drift.
func unmaskCredentials(live map[string]interface{}, prior map[string]string) map[string]string {
	out := make(map[string]string, len(live))
	for k, v := range live {
		s := fmt.Sprint(v)
		if prev, ok := prior[k]; ok && s != "" && strings.Trim(s, "*") == "" {
			out[k] = prev
			continue
		}
		out[k] = s
	}
	return out
}

func (r *cloudCredentialsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan cloudCredentialsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// The form used to go out as request headers with an empty body, so the
	// update never reached Portainer and the credentials were exposed in the
	// headers instead.
	payload, ok := r.payload(ctx, &plan, &resp.Diagnostics)
	if !ok {
		return
	}
	if err := doJSON(ctx, r.client, http.MethodPut, fmt.Sprintf("%s/cloud/credentials/%s", r.client.Endpoint, plan.ID.ValueString()), payload, nil); err != nil {
		resp.Diagnostics.AddError("Failed to update cloud credential", fmt.Sprintf("failed to update cloud credential: %s", err))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *cloudCredentialsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var id types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &id)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := doJSON(ctx, r.client, http.MethodDelete, fmt.Sprintf("%s/cloud/credentials/%s", r.client.Endpoint, id.ValueString()), nil, nil); err != nil && !isAPINotFound(err) {
		resp.Diagnostics.AddError("Failed to delete cloud credential", fmt.Sprintf("failed to delete cloud credential: %s", err))
	}
}

func (r *cloudCredentialsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
