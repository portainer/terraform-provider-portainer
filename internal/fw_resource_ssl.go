package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*sslSettingsResource)(nil)
	_ resource.ResourceWithConfigure   = (*sslSettingsResource)(nil)
	_ resource.ResourceWithImportState = (*sslSettingsResource)(nil)
)

func newSSLSettingsResource() resource.Resource { return &sslSettingsResource{} }

type sslSettingsResource struct{ client *APIClient }

type sslSettingsModel struct {
	ID          types.String `tfsdk:"id"`
	Cert        types.String `tfsdk:"cert"`
	Key         types.String `tfsdk:"key"`
	ClientCert  types.String `tfsdk:"client_cert"`
	HTTPEnabled types.Bool   `tfsdk:"http_enabled"`
}

func (r *sslSettingsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ssl"
}

func (r *sslSettingsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"cert":         schema.StringAttribute{Required: true, Sensitive: true, Description: "SSL certificate content"},
			"key":          schema.StringAttribute{Required: true, Sensitive: true, Description: "SSL private key content"},
			"client_cert":  schema.StringAttribute{Optional: true, Sensitive: true, Description: "SSL client certificate content"},
			"http_enabled": schema.BoolAttribute{Required: true, Description: "Whether HTTP should be enabled (true) or disabled (false)"},
		},
	}
}

func (r *sslSettingsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *sslSettingsResource) write(ctx context.Context, plan *sslSettingsModel) error {
	// httpenabled is always sent: Portainer keeps the current setting when it
	// is missing, so leaving false out could never switch HTTP off.
	body := struct {
		Cert        string `json:"cert,omitempty"`
		Key         string `json:"key,omitempty"`
		ClientCert  string `json:"clientCert,omitempty"`
		HTTPEnabled bool   `json:"httpenabled"`
	}{
		Cert:        plan.Cert.ValueString(),
		Key:         plan.Key.ValueString(),
		ClientCert:  plan.ClientCert.ValueString(),
		HTTPEnabled: plan.HTTPEnabled.ValueBool(),
	}
	if err := doJSON(ctx, r.client, http.MethodPut, r.client.Endpoint+"/ssl", body, nil); err != nil {
		return fmt.Errorf("failed to update SSL settings: %w", err)
	}
	plan.ID = types.StringValue("portainer-ssl")
	return nil
}

func (r *sslSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan sslSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.write(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Failed to update SSL settings", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *sslSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan sslSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.write(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Failed to update SSL settings", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *sslSettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var settings struct {
		HTTPEnabled bool `json:"httpEnabled"`
	}
	if err := doJSON(ctx, r.client, http.MethodGet, r.client.Endpoint+"/ssl", nil, &settings); err != nil {
		resp.Diagnostics.AddError("Failed to read SSL settings", fmt.Sprintf("failed to read SSL settings: %s", err))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("http_enabled"), settings.HTTPEnabled)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), "portainer-ssl")...)
}

// Delete only forgets the settings.
func (r *sslSettingsResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

func (r *sslSettingsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
