package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*alertingSettingsResource)(nil)
	_ resource.ResourceWithConfigure   = (*alertingSettingsResource)(nil)
	_ resource.ResourceWithImportState = (*alertingSettingsResource)(nil)
)

// AlertingNotificationChannel represents a notification channel within alerting settings.
type AlertingNotificationChannel struct {
	ID      int                    `json:"id,omitempty"`
	Name    string                 `json:"name,omitempty"`
	Type    string                 `json:"type,omitempty"`
	Enabled bool                   `json:"enabled,omitempty"`
	Config  map[string]interface{} `json:"config,omitempty"`
}

// AlertingSettings represents the Portainer alerting settings object.
type AlertingSettings struct {
	ID                   int                           `json:"id,omitempty"`
	Name                 string                        `json:"name,omitempty"`
	Enabled              bool                          `json:"enabled"`
	IsInternal           bool                          `json:"isInternal,omitempty"`
	URL                  string                        `json:"url,omitempty"`
	PortainerURL         string                        `json:"portainerURL,omitempty"`
	Status               string                        `json:"status,omitempty"`
	Uptime               string                        `json:"uptime,omitempty"`
	CreatedAt            string                        `json:"createdAt,omitempty"`
	CreatedBy            string                        `json:"createdBy,omitempty"`
	NotificationChannels []AlertingNotificationChannel `json:"notificationChannels,omitempty"`
}

// AlertingUpdatePayload is the body sent for PUT /observability/alerting/settings.
type AlertingUpdatePayload struct {
	AlertingSettings AlertingSettings `json:"AlertingSettings"`
}

const alertingSettingsGenericID = "portainer-alerting-settings"

func newAlertingSettingsResource() resource.Resource { return &alertingSettingsResource{} }

type alertingSettingsResource struct{ client *APIClient }

type alertingSettingsModel struct {
	ID                   types.String `tfsdk:"id"`
	Enabled              types.Bool   `tfsdk:"enabled"`
	Name                 types.String `tfsdk:"name"`
	URL                  types.String `tfsdk:"url"`
	PortainerURL         types.String `tfsdk:"portainer_url"`
	IsInternal           types.Bool   `tfsdk:"is_internal"`
	Status               types.String `tfsdk:"status"`
	Uptime               types.String `tfsdk:"uptime"`
	CreatedAt            types.String `tfsdk:"created_at"`
	CreatedBy            types.String `tfsdk:"created_by"`
	NotificationChannels types.List   `tfsdk:"notification_channels"`
}

type alertingChannelModel struct {
	ChannelID types.Int64  `tfsdk:"channel_id"`
	Name      types.String `tfsdk:"name"`
	Type      types.String `tfsdk:"type"`
	Enabled   types.Bool   `tfsdk:"enabled"`
	Config    types.Map    `tfsdk:"config"`
}

var alertingChannelAttrTypes = map[string]attr.Type{
	"channel_id": types.Int64Type,
	"name":       types.StringType,
	"type":       types.StringType,
	"enabled":    types.BoolType,
	"config":     types.MapType{ElemType: types.StringType},
}

func (r *alertingSettingsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alerting_settings"
}

func (r *alertingSettingsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	optString := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{
			Optional: true, Computed: true, Description: desc,
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		}
	}
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"enabled": schema.BoolAttribute{
				Required:    true,
				Description: "Whether alerting is enabled.",
			},
			"name":          optString("Name of the alerting settings entry."),
			"url":           optString("URL of the external AlertManager instance. Leave empty for internal."),
			"portainer_url": optString("Portainer URL used by AlertManager for callbacks."),
			"is_internal":   schema.BoolAttribute{Computed: true, Description: "Whether this uses the internal AlertManager."},
			"status":        schema.StringAttribute{Computed: true, Description: "Connection status (disabled, connected, disconnected, error)."},
			"uptime":        schema.StringAttribute{Computed: true, Description: "Uptime of the AlertManager."},
			"created_at":    schema.StringAttribute{Computed: true, Description: "Timestamp when the settings were created."},
			"created_by":    schema.StringAttribute{Computed: true, Description: "User who created the settings."},
		},
		Blocks: map[string]schema.Block{
			"notification_channels": schema.ListNestedBlock{
				Description: "List of notification channels.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"channel_id": schema.Int64Attribute{
							Optional: true, Computed: true,
							Description:   "Notification channel identifier.",
							PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
						},
						"name": optString("Name of the notification channel."),
						"type": schema.StringAttribute{
							Optional: true, Computed: true,
							Description:   "Type of the notification channel.",
							Validators:    []validator.String{stringvalidator.OneOf("slack", "webhook", "teams", "discord", "email", "pagerduty", "opsgenie")},
							PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
						},
						"enabled": schema.BoolAttribute{
							Optional: true, Computed: true,
							Description:   "Whether the notification channel is enabled.",
							PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
						},
						"config": schema.MapAttribute{
							Optional: true, Computed: true, Sensitive: true,
							ElementType:   types.StringType,
							Description:   "Configuration key-value pairs for the notification channel.",
							PlanModifiers: []planmodifier.Map{mapplanmodifier.UseStateForUnknown()},
						},
					},
				},
			},
		},
	}
}

func (r *alertingSettingsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *alertingSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan alertingSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = types.StringValue("")
	r.write(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *alertingSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state alertingSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID
	r.write(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// write PUTs the settings (create and update alike) and reads them back into
// plan.
func (r *alertingSettingsResource) write(ctx context.Context, plan *alertingSettingsModel, diags *diag.Diagnostics) {
	client := r.client
	planned := *plan
	payload := alertingSettingsPayload(ctx, *plan, diags)
	if diags.HasError() {
		return
	}

	jsonPayload, err := json.Marshal(AlertingUpdatePayload{AlertingSettings: payload})
	if err != nil {
		diags.AddError("Failed to marshal alerting settings payload", fmt.Sprintf("failed to marshal alerting settings payload: %s", err))
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, fmt.Sprintf("%s/observability/alerting/settings", client.Endpoint), bytes.NewBuffer(jsonPayload))
	if err != nil {
		diags.AddError("Failed to update alerting settings", err.Error())
		return
	}
	setAlertingAuthHeaders(req, client)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		diags.AddError("Failed to update alerting settings", err.Error())
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		diags.AddError("Failed to update alerting settings", fmt.Sprintf("failed to create/update alerting settings: %s", string(body)))
		return
	}

	// Parse the response to get the ID
	plan.ID = types.StringValue(alertingSettingsGenericID)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err == nil {
		if id, ok := result["id"].(float64); ok {
			plan.ID = types.StringValue(fmt.Sprintf("%d", int(id)))
		}
	}

	found := r.readInto(ctx, plan, false, diags)
	if diags.HasError() {
		return
	}
	if !found {
		diags.AddError("Alerting settings not found", "Portainer returned no alerting settings right after they were written")
		return
	}

	// What the configuration set is what this apply results in; should
	// Portainer report something else, the next refresh shows it (Terraform
	// rejects an apply whose result differs from its plan).
	plan.Enabled = planned.Enabled
	for _, p := range []struct{ dst, src *types.String }{
		{&plan.Name, &planned.Name}, {&plan.URL, &planned.URL}, {&plan.PortainerURL, &planned.PortainerURL},
	} {
		if !p.src.IsUnknown() {
			*p.dst = *p.src
		}
	}
	plan.NotificationChannels = alertingChannelsAfterWrite(ctx, planned.NotificationChannels, plan.NotificationChannels, diags)
}

// alertingChannelsAfterWrite keeps the planned channel blocks - their count
// has to match the configuration - and fills what the plan left unknown from
// the channels Portainer reported at the same position.
func alertingChannelsAfterWrite(ctx context.Context, planned, read types.List, diags *diag.Diagnostics) types.List {
	var want, got []alertingChannelModel
	diags.Append(planned.ElementsAs(ctx, &want, false)...)
	if !read.IsNull() && !read.IsUnknown() {
		diags.Append(read.ElementsAs(ctx, &got, false)...)
	}
	out := make([]attr.Value, 0, len(want))
	for i, ch := range want {
		var api alertingChannelModel
		if i < len(got) {
			api = got[i]
		} else {
			api = alertingChannelModel{
				ChannelID: types.Int64Value(0), Name: types.StringValue(""), Type: types.StringValue(""),
				Enabled: types.BoolValue(false), Config: types.MapValueMust(types.StringType, map[string]attr.Value{}),
			}
		}
		if ch.ChannelID.IsUnknown() {
			ch.ChannelID = api.ChannelID
		}
		if ch.Name.IsUnknown() {
			ch.Name = api.Name
		}
		if ch.Type.IsUnknown() {
			ch.Type = api.Type
		}
		if ch.Enabled.IsUnknown() {
			ch.Enabled = api.Enabled
		}
		if ch.Config.IsUnknown() {
			ch.Config = api.Config
		}
		out = append(out, alertingChannelObject(ch))
	}
	return types.ListValueMust(types.ObjectType{AttrTypes: alertingChannelAttrTypes}, out)
}

func alertingChannelObject(ch alertingChannelModel) attr.Value {
	return types.ObjectValueMust(alertingChannelAttrTypes, map[string]attr.Value{
		"channel_id": ch.ChannelID,
		"name":       ch.Name,
		"type":       ch.Type,
		"enabled":    ch.Enabled,
		"config":     ch.Config,
	})
}

func (r *alertingSettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state alertingSettingsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// An import reads every channel, as SDKv2 did (the block was
	// optional+computed there); later reads only the tracked ones.
	imported, diags := req.Private.GetKey(ctx, alertingSettingsImportedKey)
	resp.Diagnostics.Append(diags...)
	found := r.readInto(ctx, &state, len(imported) > 0, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(imported) > 0 {
		resp.Diagnostics.Append(resp.Private.SetKey(ctx, alertingSettingsImportedKey, nil)...)
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// readInto reads the settings entry m.ID names (or the first one) into m; it
// reports false when there is none.
//
// notification_channels is a block, and Terraform plans a block exactly as
// the configuration writes it. The channels Portainer reports are therefore
// only read into a state that already tracks channels, or after an import
// (all): reading them into a configuration without any would plan their
// removal on every run.
func (r *alertingSettingsResource) readInto(ctx context.Context, m *alertingSettingsModel, all bool, diags *diag.Diagnostics) bool {
	client := r.client

	// If we have a numeric ID, read the specific settings entry
	settingsID := m.ID.ValueString()
	url := fmt.Sprintf("%s/observability/alerting/settings", client.Endpoint)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		diags.AddError("Failed to read alerting settings", err.Error())
		return false
	}
	setAlertingAuthHeaders(req, client)

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		diags.AddError("Failed to read alerting settings", err.Error())
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return false
	}
	if resp.StatusCode != http.StatusOK {
		diags.AddError("Failed to read alerting settings", fmt.Sprintf("failed to read alerting settings, status: %d", resp.StatusCode))
		return false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		diags.AddError("Failed to read alerting settings", err.Error())
		return false
	}

	// The GET /observability/alerting/settings returns an array.
	// Try to find our settings entry by ID.
	var settingsList []AlertingSettings
	if err := json.Unmarshal(body, &settingsList); err != nil {
		// Try single object in case the response format differs
		var single AlertingSettings
		if err2 := json.Unmarshal(body, &single); err2 != nil {
			diags.AddError("Failed to read alerting settings", fmt.Sprintf("failed to decode alerting settings response: %s", err))
			return false
		}
		settingsList = []AlertingSettings{single}
	}

	// Find matching settings entry
	var settings *AlertingSettings
	for i := range settingsList {
		if fmt.Sprintf("%d", settingsList[i].ID) == settingsID {
			settings = &settingsList[i]
			break
		}
	}
	if settings == nil && len(settingsList) > 0 {
		// If ID is the generic one or not found, use the first entry
		settings = &settingsList[0]
		m.ID = types.StringValue(fmt.Sprintf("%d", settings.ID))
	}
	if settings == nil {
		return false
	}

	m.Enabled = types.BoolValue(settings.Enabled)
	m.Name = types.StringValue(settings.Name)
	m.URL = types.StringValue(settings.URL)
	m.PortainerURL = types.StringValue(settings.PortainerURL)
	m.IsInternal = types.BoolValue(settings.IsInternal)
	m.Status = types.StringValue(settings.Status)
	m.Uptime = types.StringValue(settings.Uptime)
	m.CreatedAt = types.StringValue(settings.CreatedAt)
	m.CreatedBy = types.StringValue(settings.CreatedBy)

	tracked := all || (!m.NotificationChannels.IsNull() && !m.NotificationChannels.IsUnknown() && len(m.NotificationChannels.Elements()) > 0)
	channels := make([]attr.Value, 0, len(settings.NotificationChannels))
	if tracked {
		for _, ch := range settings.NotificationChannels {
			cfg := make(map[string]string, len(ch.Config))
			for k, v := range ch.Config {
				cfg[k] = fmt.Sprintf("%v", v)
			}
			channels = append(channels, alertingChannelObject(alertingChannelModel{
				ChannelID: types.Int64Value(int64(ch.ID)),
				Name:      types.StringValue(ch.Name),
				Type:      types.StringValue(ch.Type),
				Enabled:   types.BoolValue(ch.Enabled),
				Config:    stringMapValue(cfg),
			}))
		}
	}
	m.NotificationChannels = types.ListValueMust(types.ObjectType{AttrTypes: alertingChannelAttrTypes}, channels)
	return true
}

func (r *alertingSettingsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state alertingSettingsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// On delete, disable alerting by sending enabled=false
	disabledSettings := AlertingSettings{Enabled: false}

	// Preserve the ID if it's numeric
	settingsID := state.ID.ValueString()
	if settingsID != "" && settingsID != alertingSettingsGenericID {
		var id int
		if _, err := fmt.Sscanf(settingsID, "%d", &id); err == nil {
			disabledSettings.ID = id
		}
	}

	if err := doJSON(ctx, r.client, http.MethodPut, fmt.Sprintf("%s/observability/alerting/settings", r.client.Endpoint), AlertingUpdatePayload{AlertingSettings: disabledSettings}, nil); err != nil {
		resp.Diagnostics.AddError("Failed to disable alerting settings", fmt.Sprintf("failed to disable alerting settings: %s", err))
	}
}

// alertingSettingsImportedKey marks, in private state, a resource imported
// and not read yet.
const alertingSettingsImportedKey = "imported"

// ImportState takes the entry ID; the read that follows fills every
// notification channel Portainer reports. Without channels in the
// configuration, the next plan then shows their removal once - harmless, as an
// empty list is not sent and Portainer keeps them.
func (r *alertingSettingsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(resp.Private.SetKey(ctx, alertingSettingsImportedKey, []byte(`true`))...)
}

// alertingSettingsPayload builds the PUT body: the planned values, where an
// optional attribute neither configured nor known yet is sent empty, as SDKv2
// did.
func alertingSettingsPayload(ctx context.Context, plan alertingSettingsModel, diags *diag.Diagnostics) AlertingSettings {
	str := func(v types.String) string {
		if v.IsNull() || v.IsUnknown() {
			return ""
		}
		return v.ValueString()
	}
	settings := AlertingSettings{
		Enabled:      plan.Enabled.ValueBool(),
		Name:         str(plan.Name),
		URL:          str(plan.URL),
		PortainerURL: str(plan.PortainerURL),
	}

	// Preserve ID for updates
	settingsID := str(plan.ID)
	if settingsID != "" && settingsID != alertingSettingsGenericID {
		var id int
		if _, err := fmt.Sscanf(settingsID, "%d", &id); err == nil {
			settings.ID = id
		}
	}

	if plan.NotificationChannels.IsNull() || plan.NotificationChannels.IsUnknown() {
		return settings
	}
	var channels []alertingChannelModel
	diags.Append(plan.NotificationChannels.ElementsAs(ctx, &channels, false)...)
	for _, ch := range channels {
		channel := AlertingNotificationChannel{
			Name: str(ch.Name),
			Type: str(ch.Type),
		}
		if !ch.ChannelID.IsNull() && !ch.ChannelID.IsUnknown() {
			channel.ID = int(ch.ChannelID.ValueInt64())
		}
		if !ch.Enabled.IsNull() && !ch.Enabled.IsUnknown() {
			channel.Enabled = ch.Enabled.ValueBool()
		}
		// SDKv2 handed every channel a config map, empty when unset.
		channel.Config = make(map[string]interface{})
		for k, v := range stringMapToGo(ctx, ch.Config, diags) {
			channel.Config[k] = v
		}
		settings.NotificationChannels = append(settings.NotificationChannels, channel)
	}
	return settings
}
