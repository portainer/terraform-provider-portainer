package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// addonConfigEntry is one stored configuration entry of an addon, in the
// shape Portainer both accepts and returns.
type addonConfigEntry struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	Sensitive bool   `json:"sensitive"`
}

var (
	_ resource.Resource                = (*addonConfigResource)(nil)
	_ resource.ResourceWithConfigure   = (*addonConfigResource)(nil)
	_ resource.ResourceWithImportState = (*addonConfigResource)(nil)
)

func newAddonConfigResource() resource.Resource { return &addonConfigResource{} }

type addonConfigResource struct{ client *APIClient }

type addonConfigModel struct {
	ID      types.String `tfsdk:"id"`
	AddonID types.String `tfsdk:"addon_id"`
	Entry   types.Set    `tfsdk:"entry"`
}

var addonConfigEntryType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"key": types.StringType, "value": types.StringType, "sensitive": types.BoolType,
}}

func (r *addonConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_addon_config"
}

func (r *addonConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"addon_id": schema.StringAttribute{
				Required:      true,
				Description:   "Catalog identifier of the addon whose stored configuration this resource owns, for example `portal-template`.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
		Blocks: map[string]schema.Block{
			"entry": schema.SetNestedBlock{
				Description: "Configuration entries of the addon. The set is authoritative: an entry removed from the configuration is deleted from the addon.",
				Validators:  []validator.Set{setvalidator.IsRequired(), setvalidator.SizeAtLeast(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							Required:    true,
							Description: "Name of the configuration entry, for example `BASE_DOMAIN`.",
						},
						// Entries marked sensitive are not returned by Portainer,
						// and any entry may legitimately hold a credential, so the
						// value never lands in a plan.
						"value": schema.StringAttribute{
							Required:    true,
							Sensitive:   true,
							Description: "Value of the configuration entry.",
						},
						"sensitive": schema.BoolAttribute{
							Optional:    true,
							Computed:    true,
							Default:     booldefault.StaticBool(false),
							Description: "Whether Portainer should treat the value as a secret, which keeps it out of API responses and makes clients display it carefully.",
						},
					},
				},
			},
		},
	}
}

func (r *addonConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

// addonConfigEntries reads an entry set, keyed for comparison between old
// and new.
func addonConfigEntries(set types.Set) map[string]addonConfigEntry {
	entries := map[string]addonConfigEntry{}
	if set.IsNull() || set.IsUnknown() {
		return entries
	}
	for _, elem := range set.Elements() {
		obj, ok := elem.(types.Object)
		if !ok {
			continue
		}
		attrs := obj.Attributes()
		key, _ := attrs["key"].(types.String)
		if key.ValueString() == "" {
			continue
		}
		value, _ := attrs["value"].(types.String)
		sensitive, _ := attrs["sensitive"].(types.Bool)
		entries[key.ValueString()] = addonConfigEntry{Key: key.ValueString(), Value: value.ValueString(), Sensitive: sensitive.ValueBool()}
	}
	return entries
}

func addonConfigURL(client *APIClient, addonID string) string {
	return addonURL(client, addonID) + "/config"
}

// addonConfigSortedKeys keeps the order of the entries sent to Portainer
// stable, so a re-apply that changes nothing produces an identical request.
func addonConfigSortedKeys(entries map[string]addonConfigEntry) []string {
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (r *addonConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m addonConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	addonID := m.AddonID.ValueString()

	// Create replaces the addon's configuration outright, which is what taking
	// ownership of it means. Updates go entry by entry instead, so a key added
	// outside Terraform is not silently destroyed on every apply.
	entries := addonConfigEntries(m.Entry)
	list := make([]addonConfigEntry, 0, len(entries))
	for _, key := range addonConfigSortedKeys(entries) {
		list = append(list, entries[key])
	}
	payload := map[string]interface{}{"entries": list}

	if err := doJSON(ctx, r.client, http.MethodPut, addonConfigURL(r.client, addonID), payload, nil); err != nil {
		addonDiagError(&resp.Diagnostics, fmt.Errorf("failed to write the configuration of addon %s: %w", addonID, err))
		return
	}

	m.ID = types.StringValue(addonID)
	r.readInto(ctx, &m, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *addonConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state addonConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	addonID := state.ID.ValueString()

	before := addonConfigEntries(state.Entry)
	after := addonConfigEntries(plan.Entry)

	for _, key := range addonConfigSortedKeys(after) {
		entry := after[key]
		if existing, ok := before[key]; ok && existing == entry {
			continue
		}
		endpoint := fmt.Sprintf("%s/%s", addonConfigURL(r.client, addonID), url.PathEscape(key))
		payload := map[string]interface{}{"value": entry.Value, "sensitive": entry.Sensitive}
		if err := doJSON(ctx, r.client, http.MethodPatch, endpoint, payload, nil); err != nil {
			addonDiagError(&resp.Diagnostics, fmt.Errorf("failed to set configuration entry %q of addon %s: %w", key, addonID, err))
			return
		}
	}

	for _, key := range addonConfigSortedKeys(before) {
		if _, ok := after[key]; ok {
			continue
		}
		endpoint := fmt.Sprintf("%s/%s", addonConfigURL(r.client, addonID), url.PathEscape(key))
		if err := doJSON(ctx, r.client, http.MethodDelete, endpoint, nil, nil); err != nil && !isAPINotFound(err) {
			addonDiagError(&resp.Diagnostics, fmt.Errorf("failed to remove configuration entry %q of addon %s: %w", key, addonID, err))
			return
		}
	}

	plan.ID = state.ID
	r.readInto(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// readInto reads the configuration back after a write, as the SDKv2 version
// did. The state records the entries just applied: Portainer withholds
// sensitive values and may hold keys added outside Terraform, and the next
// refresh reports either.
func (r *addonConfigResource) readInto(ctx context.Context, m *addonConfigModel, diags *diag.Diagnostics) {
	var response struct {
		Entries []addonConfigEntry `json:"entries"`
	}
	addonID := m.ID.ValueString()
	if err := doJSON(ctx, r.client, http.MethodGet, addonConfigURL(r.client, addonID), nil, &response); err != nil && !isAPINotFound(err) {
		addonDiagError(diags, fmt.Errorf("failed to read the configuration of addon %s: %w", addonID, err))
	}
}

func (r *addonConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m addonConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := m.ID.ValueString()

	var response struct {
		Entries []addonConfigEntry `json:"entries"`
	}
	if err := doJSON(ctx, r.client, http.MethodGet, addonConfigURL(r.client, id), nil, &response); err != nil {
		if isAPINotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		addonDiagError(&resp.Diagnostics, fmt.Errorf("failed to read the configuration of addon %s: %w", id, err))
		return
	}

	// Portainer withholds the value of a sensitive entry. Taking the response
	// at face value would blank it in state and make the next plan look like a
	// change, so the known value is kept whenever none came back.
	known := addonConfigEntries(m.Entry)
	elems := make([]attr.Value, 0, len(response.Entries))
	for _, entry := range response.Entries {
		if entry.Value == "" {
			if previous, ok := known[entry.Key]; ok {
				entry.Value = previous.Value
			}
		}
		elems = append(elems, types.ObjectValueMust(addonConfigEntryType.AttrTypes, map[string]attr.Value{
			"key":       types.StringValue(entry.Key),
			"value":     types.StringValue(entry.Value),
			"sensitive": types.BoolValue(entry.Sensitive),
		}))
	}

	m.AddonID = types.StringValue(id)
	m.Entry = types.SetValueMust(addonConfigEntryType, elems)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *addonConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m addonConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := m.ID.ValueString()
	if err := doJSON(ctx, r.client, http.MethodDelete, addonConfigURL(r.client, id), nil, nil); err != nil && !isAPINotFound(err) {
		addonDiagError(&resp.Diagnostics, fmt.Errorf("failed to clear the configuration of addon %s: %w", id, err))
	}
}

func (r *addonConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
