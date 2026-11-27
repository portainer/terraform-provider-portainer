package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*alertingSilenceResource)(nil)
	_ resource.ResourceWithConfigure   = (*alertingSilenceResource)(nil)
	_ resource.ResourceWithImportState = (*alertingSilenceResource)(nil)
)

// SilenceMatcher represents a matcher within an AlertManager silence.
type SilenceMatcher struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	IsRegex bool   `json:"isRegex"`
	IsEqual bool   `json:"isEqual,omitempty"`
}

// PostableSilence represents the silence object sent to the AlertManager.
type PostableSilence struct {
	ID        string           `json:"id,omitempty"`
	Comment   string           `json:"comment"`
	CreatedBy string           `json:"createdBy"`
	StartsAt  string           `json:"startsAt"`
	EndsAt    string           `json:"endsAt"`
	Matchers  []SilenceMatcher `json:"matchers"`
}

// CreateSilencePayload is the body sent for POST /observability/alerting/silence.
type CreateSilencePayload struct {
	AlertManagerURL string          `json:"alertManagerURL"`
	Silence         PostableSilence `json:"silence"`
}

func newAlertingSilenceResource() resource.Resource { return &alertingSilenceResource{} }

type alertingSilenceResource struct{ client *APIClient }

type alertingSilenceModel struct {
	ID              types.String `tfsdk:"id"`
	AlertManagerURL types.String `tfsdk:"alert_manager_url"`
	Comment         types.String `tfsdk:"comment"`
	CreatedBy       types.String `tfsdk:"created_by"`
	StartsAt        types.String `tfsdk:"starts_at"`
	EndsAt          types.String `tfsdk:"ends_at"`
	Matchers        types.List   `tfsdk:"matchers"`
}

type alertingSilenceMatcherModel struct {
	Name    types.String `tfsdk:"name"`
	Value   types.String `tfsdk:"value"`
	IsRegex types.Bool   `tfsdk:"is_regex"`
	IsEqual types.Bool   `tfsdk:"is_equal"`
}

func (r *alertingSilenceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alerting_silence"
}

func (r *alertingSilenceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	forceNew := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{
			Required: true, Description: desc,
			PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
		}
	}
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"alert_manager_url": forceNew("URL of the AlertManager instance."),
			"comment":           forceNew("Comment explaining the reason for the silence."),
			"created_by":        forceNew("Name of the user creating the silence."),
			"starts_at":         forceNew("Start time of the silence in RFC3339 format."),
			"ends_at":           forceNew("End time of the silence in RFC3339 format."),
		},
		Blocks: map[string]schema.Block{
			"matchers": schema.ListNestedBlock{
				Description:   "List of matchers to determine which alerts are silenced.",
				Validators:    []validator.List{listvalidator.IsRequired(), listvalidator.SizeAtLeast(1)},
				PlanModifiers: []planmodifier.List{listplanmodifier.RequiresReplace()},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name":     schema.StringAttribute{Required: true, Description: "Label name to match."},
						"value":    schema.StringAttribute{Required: true, Description: "Label value to match."},
						"is_regex": schema.BoolAttribute{Required: true, Description: "Whether the value is a regular expression."},
						"is_equal": schema.BoolAttribute{
							Optional: true, Computed: true,
							Default:     booldefault.StaticBool(true),
							Description: "Whether to match for equality (true) or inequality (false).",
						},
					},
				},
			},
		},
	}
}

func (r *alertingSilenceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *alertingSilenceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan alertingSilenceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	silence := PostableSilence{
		Comment:   plan.Comment.ValueString(),
		CreatedBy: plan.CreatedBy.ValueString(),
		StartsAt:  plan.StartsAt.ValueString(),
		EndsAt:    plan.EndsAt.ValueString(),
	}
	var matchers []alertingSilenceMatcherModel
	resp.Diagnostics.Append(plan.Matchers.ElementsAs(ctx, &matchers, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	for _, m := range matchers {
		silence.Matchers = append(silence.Matchers, SilenceMatcher{
			Name:    m.Name.ValueString(),
			Value:   m.Value.ValueString(),
			IsRegex: m.IsRegex.ValueBool(),
			IsEqual: m.IsEqual.ValueBool(),
		})
	}

	payload := CreateSilencePayload{
		AlertManagerURL: plan.AlertManagerURL.ValueString(),
		Silence:         silence,
	}

	var result map[string]interface{}
	if err := doJSON(ctx, r.client, http.MethodPost, fmt.Sprintf("%s/observability/alerting/silence", r.client.Endpoint), payload, &result); err != nil {
		resp.Diagnostics.AddError("Failed to create alert silence", fmt.Sprintf("failed to create alert silence: %s", err))
		return
	}

	if silenceID, ok := result["silenceID"].(string); ok && silenceID != "" {
		plan.ID = types.StringValue(silenceID)
	} else if id, ok := result["id"].(string); ok && id != "" {
		plan.ID = types.StringValue(id)
	} else {
		resp.Diagnostics.AddError("Failed to create alert silence", "silence created but no ID returned in response")
		return
	}

	if err := r.check(ctx); err != nil {
		resp.Diagnostics.AddError("Failed to read alert silence", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// check asks for the silenced alerts, as SDKv2 did. The endpoint returns
// silenced alerts rather than silences, so the answer cannot tell whether the
// silence still exists: only a failure to send the request is an error, and
// the stored attributes stand (every one forces a new silence anyway).
func (r *alertingSilenceResource) check(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/observability/alerting/alerts", r.client.Endpoint), nil)
	if err != nil {
		return err
	}
	setAlertingAuthHeaders(req, r.client)

	q := req.URL.Query()
	q.Set("status", "silenced")
	req.URL.RawQuery = q.Encode()

	resp, err := r.client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	return nil
}

func (r *alertingSilenceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if err := r.check(ctx); err != nil {
		resp.Diagnostics.AddError("Failed to read alert silence", err.Error())
	}
}

// Update is never called: every attribute requires replacement.
func (r *alertingSilenceResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Alert silences cannot be updated", "Every attribute of portainer_alerting_silence forces a new resource.")
}

func (r *alertingSilenceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state alertingSilenceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	q := url.Values{}
	q.Set("alertManagerURL", state.AlertManagerURL.ValueString())
	deleteURL := fmt.Sprintf("%s/observability/alerting/silence/%s?%s", r.client.Endpoint, state.ID.ValueString(), q.Encode())

	if err := doJSON(ctx, r.client, http.MethodDelete, deleteURL, nil, nil); err != nil && !isAPINotFound(err) {
		resp.Diagnostics.AddError("Failed to delete alert silence", fmt.Sprintf("failed to delete alert silence %s: %s", state.ID.ValueString(), err))
	}
}

func (r *alertingSilenceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
