package internal

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// LicensePayload is the body of POST /licenses/add.
type LicensePayload struct {
	Key string `json:"key"`
}

// LicenseResponse is the answer of POST /licenses/add.
type LicenseResponse struct {
	ConflictingKeys []string `json:"conflictingKeys"`
}

var (
	_ resource.Resource                     = (*licensesResource)(nil)
	_ resource.ResourceWithConfigure        = (*licensesResource)(nil)
	_ resource.ResourceWithImportState      = (*licensesResource)(nil)
	_ resource.ResourceWithConfigValidators = (*licensesResource)(nil)
)

func newLicensesResource() resource.Resource { return &licensesResource{} }

type licensesResource struct{ client *APIClient }

type licensesModel struct {
	ID              types.String `tfsdk:"id"`
	Key             types.String `tfsdk:"key"`
	KeyWO           types.String `tfsdk:"key_wo"`
	KeyWOVersion    types.Int64  `tfsdk:"key_wo_version"`
	Force           types.Bool   `tfsdk:"force"`
	ConflictingKeys types.List   `tfsdk:"conflicting_keys"`
}

func (r *licensesResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_licenses"
}

func (r *licensesResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"key": schema.StringAttribute{
				Optional:      true,
				Sensitive:     true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "License key to be attached. Stored in state as a sensitive value; use `key_wo` to keep it out of state.",
			},
			"key_wo":         gsbWriteOnlySecretAttribute("key", "key_wo_version", "license key to be attached"),
			"key_wo_version": gsbWriteOnlyVersionAttribute("key_wo", true),
			"force": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Default:       booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
				Description:   "Force attach even if there are conflicting licenses",
			},
			"conflicting_keys": schema.ListAttribute{
				Computed:      true,
				ElementType:   types.StringType,
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
				Description:   "List of conflicting license keys, if any",
			},
		},
	}
}

func (r *licensesResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(path.MatchRoot("key"), path.MatchRoot("key_wo")),
	}
}

func (r *licensesResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *licensesResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan licensesModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	licenseKey, _ := gsbTwinSecret(ctx, req.Config, plan.Key, path.Root("key_wo"), plan.KeyWOVersion, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	url := "/licenses/add"
	if plan.Force.ValueBool() {
		url += "?force=true"
	}
	var result LicenseResponse
	if err := doJSON(ctx, r.client, http.MethodPost, r.client.Endpoint+url, LicensePayload{Key: licenseKey}, &result); err != nil {
		resp.Diagnostics.AddError("Failed to attach license", fmt.Sprintf("failed to attach license: %s", err))
		return
	}

	elems := make([]attr.Value, 0, len(result.ConflictingKeys))
	for _, k := range result.ConflictingKeys {
		elems = append(elems, types.StringValue(k))
	}
	plan.ConflictingKeys = types.ListValueMust(types.StringType, elems)
	plan.ID = types.StringValue(fmt.Sprintf("%x", sha256.Sum256([]byte(licenseKey))))
	plan.KeyWO = types.StringNull()
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// attachedLicenseKey returns the attached license whose SHA-256 is the
// resource ID, or "" when it is not attached any more. The ID is all there is
// to go by after an import or with key_wo, which leave key empty.
func attachedLicenseKey(ctx context.Context, client *APIClient, id string) (string, error) {
	var licenses []struct {
		LicenseKey string `json:"licenseKey"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/licenses", nil, &licenses); err != nil {
		return "", fmt.Errorf("failed to get licenses: %w", err)
	}
	for _, lic := range licenses {
		if fmt.Sprintf("%x", sha256.Sum256([]byte(lic.LicenseKey))) == id {
			return lic.LicenseKey, nil
		}
	}
	return "", nil
}

func (r *licensesResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var id types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &id)...)
	if resp.Diagnostics.HasError() {
		return
	}
	key, err := attachedLicenseKey(ctx, r.client, id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read license", err.Error())
		return
	}
	if key == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	// An imported license has no force flag yet: it takes the default.
	var force types.Bool
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("force"), &force)...)
	if force.IsNull() {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("force"), false)...)
	}
	var conflicting types.List
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("conflicting_keys"), &conflicting)...)
	if conflicting.IsNull() {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("conflicting_keys"), []string{})...)
	}
}

// Update is never called: every attribute requires replacement.
func (r *licensesResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Licenses cannot be updated", "Every attribute of portainer_licenses forces a new resource.")
}

func (r *licensesResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var id types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &id)...)
	if resp.Diagnostics.HasError() {
		return
	}
	key, err := attachedLicenseKey(ctx, r.client, id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete license", err.Error())
		return
	}
	if key == "" {
		return
	}
	payload := map[string]interface{}{"licenseKeys": []string{key}}
	if err := doJSON(ctx, r.client, http.MethodPost, r.client.Endpoint+"/licenses/remove", payload, nil); err != nil {
		resp.Diagnostics.AddError("Failed to delete license", fmt.Sprintf("failed to send license removal request: %s", err))
	}
}

func (r *licensesResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
