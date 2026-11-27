package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// The single-switch settings resources: portainer_settings_default_registry,
// portainer_settings_additional_functionality and
// portainer_settings_experimental. Each manages one bool of a settings
// endpoint under a fixed ID; deleting one only stops managing the setting.

// settingsToggle describes one of them.
type settingsToggle struct {
	typeName    string
	id          string
	attr        string
	description string
	// optional makes the attribute optional with a false default rather
	// than required.
	optional bool
	// readAfterWrite reads the setting back after writing it, as the SDKv2
	// version did.
	readAfterWrite bool
	// extra is an optional second switch of the same endpoint, added after
	// v3.0.0: sent only when configured (write gets nil otherwise) and read
	// back only when the state has it, so existing states stay unchanged.
	extra            string
	extraDescription string
	// extraNotReported is set when the server accepts the extra switch but
	// does not report it back: the state then keeps the configured value.
	extraNotReported bool
	// attrNotReported is the same for the main switch; attrDeprecation marks
	// it deprecated.
	attrNotReported bool
	attrDeprecation string
	write           func(ctx context.Context, c *APIClient, v bool, extra *bool) error
	// read returns the switch and the extra one (false without one).
	read func(ctx context.Context, c *APIClient) (bool, bool, error)
}

var (
	_ resource.Resource                = (*settingsToggleResource)(nil)
	_ resource.ResourceWithConfigure   = (*settingsToggleResource)(nil)
	_ resource.ResourceWithImportState = (*settingsToggleResource)(nil)
)

type settingsToggleResource struct {
	client *APIClient
	t      settingsToggle
}

// resourceSettingsDefaultRegistry controls whether Portainer offers the
// built-in anonymous Docker Hub registry when a user picks an image source.
// It has its own endpoint rather than being part of PUT /settings, so it is
// its own resource; folding it into portainer_settings would mean every
// settings apply also rewrote it.
func newSettingsDefaultRegistryResource() resource.Resource {
	return &settingsToggleResource{t: settingsToggle{
		typeName:       "_settings_default_registry",
		id:             "portainer-settings-default-registry",
		attr:           "hide",
		description:    "Whether to hide the built-in anonymous Docker Hub registry from the image source pickers, so users can only deploy from registries you have configured.",
		readAfterWrite: true,
		write: func(ctx context.Context, c *APIClient, v bool, _ *bool) error {
			if err := doJSON(ctx, c, http.MethodPut, c.Endpoint+"/settings/default_registry", map[string]interface{}{"Hide": v}, nil); err != nil {
				return fmt.Errorf("failed to update the default registry settings: %w", err)
			}
			return nil
		},
		read: func(ctx context.Context, c *APIClient) (bool, bool, error) {
			// The endpoint is write-only, so the current value is read from the
			// main settings object, which is where Portainer stores it.
			var settings struct {
				DefaultRegistry struct {
					Hide bool `json:"Hide"`
				} `json:"DefaultRegistry"`
			}
			if err := doJSON(ctx, c, http.MethodGet, c.Endpoint+"/settings", nil, &settings); err != nil {
				return false, false, fmt.Errorf("failed to read the default registry settings: %w", err)
			}
			return settings.DefaultRegistry.Hide, false, nil
		},
	}}
}

// newSettingsAdditionalFunctionalityResource switches the optional Portainer
// features on and off. Today that is the policy engine, which is what gates
// the portainer_policy resources.
func newSettingsAdditionalFunctionalityResource() resource.Resource {
	return &settingsToggleResource{t: settingsToggle{
		typeName:       "_settings_additional_functionality",
		id:             "portainer-settings-additional-functionality",
		attr:           "policies",
		description:    "Whether the policy engine is enabled. The `portainer_policy` resources need it switched on.",
		readAfterWrite: true,
		write: func(ctx context.Context, c *APIClient, v bool, _ *bool) error {
			if err := doJSON(ctx, c, http.MethodPut, c.Endpoint+"/settings/additional_functionality", map[string]interface{}{"Policies": v}, nil); err != nil {
				return fmt.Errorf("failed to update the additional functionality settings: %w", err)
			}
			return nil
		},
		read: func(ctx context.Context, c *APIClient) (bool, bool, error) {
			var response struct {
				AdditionalFunctionality struct {
					Policies bool `json:"Policies"`
				} `json:"additionalFunctionality"`
			}
			if err := doJSON(ctx, c, http.MethodGet, c.Endpoint+"/settings/additional_functionality", nil, &response); err != nil {
				return false, false, fmt.Errorf("failed to read the additional functionality settings: %w", err)
			}
			return response.AdditionalFunctionality.Policies, false, nil
		},
	}}
}

// ExperimentalSettingsPayload is the body of PUT /settings/experimental.
type ExperimentalSettingsPayload struct {
	OpenAIIntegration bool `json:"openAIIntegration"`
	// FleetManagement is only sent when fleet_management is configured.
	FleetManagement *bool `json:"fleetManagement,omitempty"`
}

func newExperimentalSettingsResource() resource.Resource {
	return &settingsToggleResource{t: settingsToggle{
		typeName:    "_settings_experimental",
		id:          "portainer-experimental-settings",
		attr:        "openai_integration",
		description: "Enable or disable OpenAI integration.",
		optional:    true,
		extra:       "fleet_management",
		extraDescription: "Enable or disable the experimental fleet management feature. Business Edition only. " +
			"Left out, Portainer keeps its current value. Portainer does not report the setting back, so changes made outside Terraform are not detected.",
		// GET /settings/experimental answers {"experimentalFeatures":{}} on
		// Business Edition 2.39 to 3.1, whatever was written, and the update
		// payload has no openAIIntegration any more: Portainer dropped its
		// OpenAI integration.
		extraNotReported: true,
		attrNotReported:  true,
		attrDeprecation: "openai_integration is deprecated: Portainer 2.39 and later have no OpenAI integration and ignore it. " +
			"It will be removed in a future major version.",
		write: func(ctx context.Context, c *APIClient, v bool, fleet *bool) error {
			if err := doJSON(ctx, c, http.MethodPut, fmt.Sprintf("%s/settings/experimental", c.Endpoint), ExperimentalSettingsPayload{OpenAIIntegration: v, FleetManagement: fleet}, nil); err != nil {
				return fmt.Errorf("failed to apply experimental settings: %w", err)
			}
			return nil
		},
		read: func(ctx context.Context, c *APIClient) (bool, bool, error) {
			var result struct {
				ExperimentalFeatures struct {
					OpenAIIntegration bool `json:"OpenAIIntegration"`
					FleetManagement   bool `json:"FleetManagement"`
				} `json:"experimentalFeatures"`
			}
			if err := doJSON(ctx, c, http.MethodGet, fmt.Sprintf("%s/settings/experimental", c.Endpoint), nil, &result); err != nil {
				return false, false, fmt.Errorf("failed to retrieve experimental settings: %w", err)
			}
			return result.ExperimentalFeatures.OpenAIIntegration, result.ExperimentalFeatures.FleetManagement, nil
		},
	}}
}

func (r *settingsToggleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + r.t.typeName
}

func (r *settingsToggleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	value := schema.BoolAttribute{Required: true, Description: r.t.description, DeprecationMessage: r.t.attrDeprecation}
	if r.t.optional {
		value = schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: r.t.description, DeprecationMessage: r.t.attrDeprecation}
	}
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			r.t.attr: value,
		},
	}
	if r.t.extra != "" {
		resp.Schema.Attributes[r.t.extra] = schema.BoolAttribute{Optional: true, Description: r.t.extraDescription}
	}
}

// settingsToggleSource is a plan or state.
type settingsToggleSource interface {
	GetAttribute(ctx context.Context, p path.Path, target interface{}) diag.Diagnostics
}

// extraValue reads the extra switch from a plan or state: nil when there is
// none or it is null.
func (r *settingsToggleResource) extraValue(ctx context.Context, src settingsToggleSource, diags *diag.Diagnostics) *bool {
	if r.t.extra == "" {
		return nil
	}
	var v types.Bool
	diags.Append(src.GetAttribute(ctx, path.Root(r.t.extra), &v)...)
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return v.ValueBoolPointer()
}

func (r *settingsToggleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *settingsToggleResource) write(ctx context.Context, v bool, extra *bool, state *tfsdk.State, diags *diag.Diagnostics) {
	if err := r.t.write(ctx, r.client, v, extra); err != nil {
		diags.AddError("Failed to update the setting", err.Error())
		return
	}
	if r.t.readAfterWrite {
		// The read is kept for its error: a value that differs shows on the
		// next refresh, as the framework rejects a changed configured value.
		if _, _, err := r.t.read(ctx, r.client); err != nil {
			diags.AddError("Failed to read the setting", err.Error())
			return
		}
	}
	diags.Append(state.SetAttribute(ctx, path.Root("id"), r.t.id)...)
	diags.Append(state.SetAttribute(ctx, path.Root(r.t.attr), v)...)
	if r.t.extra != "" {
		diags.Append(state.SetAttribute(ctx, path.Root(r.t.extra), extra)...)
	}
}

func (r *settingsToggleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var v types.Bool
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root(r.t.attr), &v)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, v.ValueBool(), r.extraValue(ctx, req.Plan, &resp.Diagnostics), &resp.State, &resp.Diagnostics)
}

func (r *settingsToggleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var v types.Bool
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root(r.t.attr), &v)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, v.ValueBool(), r.extraValue(ctx, req.Plan, &resp.Diagnostics), &resp.State, &resp.Diagnostics)
}

func (r *settingsToggleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	v, extra, err := r.t.read(ctx, r.client)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read the setting", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), r.t.id)...)
	if !r.t.attrNotReported {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(r.t.attr), v)...)
	} else {
		// Not reported: keep the state's value, and after an import (null)
		// the default, which is what a server without the feature has.
		var cur types.Bool
		resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root(r.t.attr), &cur)...)
		if cur.IsNull() || cur.IsUnknown() {
			resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(r.t.attr), false)...)
		}
	}
	// The extra switch is read back only while it is managed (not null).
	if !r.t.extraNotReported && r.extraValue(ctx, req.State, &resp.Diagnostics) != nil {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(r.t.extra), extra)...)
	}
}

// Delete only forgets the setting, which always has a value.
func (r *settingsToggleResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

func (r *settingsToggleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
