package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource              = (*registryConfigureResource)(nil)
	_ resource.ResourceWithConfigure = (*registryConfigureResource)(nil)
)

func newRegistryConfigureResource() resource.Resource { return &registryConfigureResource{} }

type registryConfigureResource struct{ client *APIClient }

type registryConfigureModel struct {
	ID                types.String `tfsdk:"id"`
	RegistryID        types.Int64  `tfsdk:"registry_id"`
	Authentication    types.Bool   `tfsdk:"authentication"`
	Username          types.String `tfsdk:"username"`
	Password          types.String `tfsdk:"password"`
	PasswordWO        types.String `tfsdk:"password_wo"`
	PasswordWOVersion types.Int64  `tfsdk:"password_wo_version"`
	Region            types.String `tfsdk:"region"`
	TLS               types.Bool   `tfsdk:"tls"`
	TLSSkipVerify     types.Bool   `tfsdk:"tls_skip_verify"`
	TLSCACert         types.String `tfsdk:"tls_ca_cert"`
	TLSCert           types.String `tfsdk:"tls_cert"`
	TLSKey            types.String `tfsdk:"tls_key"`
	TLSKeyWO          types.String `tfsdk:"tls_key_wo"`
	TLSKeyWOVersion   types.Int64  `tfsdk:"tls_key_wo_version"`
}

func (r *registryConfigureResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_registry_configure"
}

// registryConfigureWriteOnly is the write-only twin of secretKey, as
// writeOnlySecretSchema declares it for SDKv2 resources.
func registryConfigureWriteOnly(secretKey, versionKey, what string) schema.StringAttribute {
	return schema.StringAttribute{
		Optional:  true,
		Sensitive: true,
		WriteOnly: true,
		Validators: []validator.String{
			stringvalidator.ConflictsWith(path.MatchRoot(secretKey)),
			stringvalidator.AlsoRequires(path.MatchRoot(versionKey)),
		},
		Description: fmt.Sprintf("Write-only %s (supports ephemeral values; never stored in Terraform state). Conflicts with `%s`. Requires Terraform 1.11 or later.", what, secretKey),
	}
}

func (r *registryConfigureResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the configuration: <registry_id>/configure.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"registry_id": schema.Int64Attribute{
				Required:      true,
				Description:   "Identifier of the registry to configure.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"authentication": schema.BoolAttribute{
				Required:    true,
				Description: "Whether the registry requires authentication. When false, Portainer ignores `username` and `password`.",
			},
			"username": schema.StringAttribute{
				Optional:    true,
				Description: "Username used to authenticate against the registry. Required when `authentication` is true.",
			},
			"password": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Password or token paired with `username`. Portainer never returns it, so it cannot be checked for drift. Stored in state as a sensitive value; use `password_wo` to keep it out of state.",
				Validators:  []validator.String{stringvalidator.ConflictsWith(path.MatchRoot("password_wo"))},
			},
			"password_wo": registryConfigureWriteOnly("password", "password_wo_version", "registry password"),
			"password_wo_version": schema.Int64Attribute{
				Optional:    true,
				Description: "Version of `password_wo`. Increment it to send a new value.",
			},
			"region": schema.StringAttribute{
				Optional:    true,
				Description: "AWS region of an ECR registry. Ignored by other registry types.",
			},
			"tls": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether Portainer contacts the registry over TLS.",
			},
			"tls_skip_verify": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether to skip verification of the registry's TLS certificate.",
			},
			"tls_ca_cert": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "PEM-encoded CA certificate used to verify the registry's certificate. Stored in state as a sensitive value.",
			},
			"tls_cert": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "PEM-encoded client certificate presented to the registry. Stored in state as a sensitive value.",
			},
			"tls_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "PEM-encoded client private key paired with `tls_cert`. Stored in state as a sensitive value; use `tls_key_wo` to keep it out of state.",
				Validators:  []validator.String{stringvalidator.ConflictsWith(path.MatchRoot("tls_key_wo"))},
			},
			"tls_key_wo": registryConfigureWriteOnly("tls_key", "tls_key_wo_version", "PEM-encoded TLS client key"),
			"tls_key_wo_version": schema.Int64Attribute{
				Optional:    true,
				Description: "Version of `tls_key_wo`. Increment it to send a new value.",
			},
		},
	}
}

func (r *registryConfigureResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

// registryConfigureSecret returns the secret, or its write-only twin while
// the version is set, as twinSecret does for SDKv2 resources.
func registryConfigureSecret(value types.String, version types.Int64, wo string) string {
	if version.ValueInt64() != 0 && wo != "" {
		return wo
	}
	return value.ValueString()
}

func (r *registryConfigureResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan registryConfigureModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.configure(ctx, &plan, req.Config, &resp.State, &resp.Diagnostics)
}

// Update posts the whole configuration again, as Create does.
func (r *registryConfigureResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan registryConfigureModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.configure(ctx, &plan, req.Config, &resp.State, &resp.Diagnostics)
}

func (r *registryConfigureResource) configure(ctx context.Context, plan *registryConfigureModel, config tfsdk.Config, state *tfsdk.State, diags *diag.Diagnostics) {
	registryID := plan.RegistryID.ValueInt64()
	passwordWO := writeOnlyConfigString(ctx, config, path.Root("password_wo"), diags)
	tlsKeyWO := writeOnlyConfigString(ctx, config, path.Root("tls_key_wo"), diags)
	if diags.HasError() {
		return
	}

	// registries.registryConfigurePayload uses PascalCase field names. The three
	// TLS files are []byte on the Portainer side, which encoding/json represents
	// as base64 - Go's marshaller does that conversion for a []byte value, so
	// the PEM text is passed through as-is here.
	payload := map[string]interface{}{
		"Authentication": plan.Authentication.ValueBool(),
		"TLS":            plan.TLS.ValueBool(),
		"TLSSkipVerify":  plan.TLSSkipVerify.ValueBool(),
	}
	if v := plan.Username.ValueString(); v != "" {
		payload["Username"] = v
	}
	if v := plan.Region.ValueString(); v != "" {
		payload["Region"] = v
	}
	if password := registryConfigureSecret(plan.Password, plan.PasswordWOVersion, passwordWO); password != "" {
		payload["Password"] = password
	}
	if v := plan.TLSCACert.ValueString(); v != "" {
		payload["TLSCACertFile"] = []byte(v)
	}
	if v := plan.TLSCert.ValueString(); v != "" {
		payload["TLSCertFile"] = []byte(v)
	}
	if key := registryConfigureSecret(plan.TLSKey, plan.TLSKeyWOVersion, tlsKeyWO); key != "" {
		payload["TLSKeyFile"] = []byte(key)
	}

	url := fmt.Sprintf("%s/registries/%d/configure", r.client.Endpoint, registryID)
	if err := doJSON(ctx, r.client, http.MethodPost, url, payload, nil); err != nil {
		diags.AddError("Failed to configure registry", fmt.Errorf("failed to configure registry %d: %w", registryID, err).Error())
		return
	}

	plan.ID = types.StringValue(fmt.Sprintf("%d/configure", registryID))
	diags.Append(state.Set(ctx, plan)...)
}

// Read has nothing to refresh: Portainer has no endpoint to read a registry
// configuration back.
func (r *registryConfigureResource) Read(context.Context, resource.ReadRequest, *resource.ReadResponse) {
}

// Delete only stops managing the configuration: Portainer has no endpoint to
// clear it, so the registry stays as configured.
func (r *registryConfigureResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}
