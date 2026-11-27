package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ssrfAllowListKey is the only allow list Portainer defines
// (portainer.AllowListKey has the single value AllowListSSRF). The resource
// therefore has no argument for it: an attribute with exactly one legal value
// is noise in every configuration that has to write it out.
const ssrfAllowListKey = 0

// ssrfModes maps the readable mode names this resource takes onto the integers
// Portainer stores (portainer.SSRFMode).
var ssrfModes = map[string]int{
	"off":     0,
	"audit":   1,
	"enforce": 2,
}

// ssrfModeNames is the inverse, for reading state back.
var ssrfModeNames = map[int]string{
	0: "off",
	1: "audit",
	2: "enforce",
}

const ssrfAllowListID = "portainer-ssrf-allowlist"

var (
	_ resource.Resource                = (*ssrfAllowListResource)(nil)
	_ resource.ResourceWithConfigure   = (*ssrfAllowListResource)(nil)
	_ resource.ResourceWithImportState = (*ssrfAllowListResource)(nil)
)

// newSSRFAllowListResource manages the list of URLs Portainer will make
// outbound proxy requests to, and how strictly the list is applied.
func newSSRFAllowListResource() resource.Resource { return &ssrfAllowListResource{} }

type ssrfAllowListResource struct{ client *APIClient }

type ssrfAllowListModel struct {
	ID      types.String `tfsdk:"id"`
	Mode    types.String `tfsdk:"mode"`
	Entries types.List   `tfsdk:"entries"`
}

func (r *ssrfAllowListResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ssrf_allowlist"
}

func (r *ssrfAllowListResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"mode": schema.StringAttribute{
				Required:    true,
				Validators:  []validator.String{stringvalidator.OneOf("off", "audit", "enforce")},
				Description: "How the allow list is applied: `off` does not check outbound requests at all, `audit` records requests that are not on the list but still makes them, and `enforce` blocks them.",
			},
			"entries": schema.ListAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "URLs Portainer is allowed to make outbound proxy requests to. The list is authoritative: a URL missing from it is removed.",
			},
		},
	}
}

func (r *ssrfAllowListResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func ssrfAllowListURL(client *APIClient) string {
	return fmt.Sprintf("%s/allowlist/%d", client.Endpoint, ssrfAllowListKey)
}

func (r *ssrfAllowListResource) write(ctx context.Context, plan *ssrfAllowListModel, diags *diag.Diagnostics) {
	list := []string{}
	if !plan.Entries.IsNull() && !plan.Entries.IsUnknown() {
		diags.Append(plan.Entries.ElementsAs(ctx, &list, false)...)
		if diags.HasError() {
			return
		}
	}

	// The endpoint replaces the list outright, so an empty one is a meaningful
	// request rather than something to omit: it allows nothing. Paired with
	// mode `enforce` that blocks every outbound proxy request, which is why the
	// mode is spelled out rather than defaulted.
	payload := map[string]interface{}{
		"Entries": list,
		"Mode":    ssrfModes[plan.Mode.ValueString()],
	}
	if err := doJSON(ctx, r.client, http.MethodPut, ssrfAllowListURL(r.client), payload, nil); err != nil {
		diags.AddError("Failed to update the SSRF allow list", fmt.Sprintf("failed to update the SSRF allow list: %s", err))
		return
	}
	plan.ID = types.StringValue(ssrfAllowListID)
	// The read after the write is kept for its errors; the configured values
	// are what was written.
	read := *plan
	r.read(ctx, &read, diags)
}

func (r *ssrfAllowListResource) read(ctx context.Context, m *ssrfAllowListModel, diags *diag.Diagnostics) {
	var allowList struct {
		Entries []string `json:"Entries"`
		Mode    int      `json:"Mode"`
	}
	if err := doJSON(ctx, r.client, http.MethodGet, ssrfAllowListURL(r.client), nil, &allowList); err != nil {
		diags.AddError("Failed to read the SSRF allow list", fmt.Sprintf("failed to read the SSRF allow list: %s", err))
		return
	}

	mode, ok := ssrfModeNames[allowList.Mode]
	if !ok {
		// A mode this provider does not know about is worth saying out loud:
		// silently mapping it onto "off" would misreport a security control.
		diags.AddError("Unrecognised SSRF mode", fmt.Sprintf("unrecognised SSRF mode %d reported by Portainer", allowList.Mode))
		return
	}

	m.ID = types.StringValue(ssrfAllowListID)
	m.Mode = types.StringValue(mode)
	elems := make([]attr.Value, 0, len(allowList.Entries))
	for _, e := range allowList.Entries {
		elems = append(elems, types.StringValue(e))
	}
	m.Entries = types.ListValueMust(types.StringType, elems)
}

func (r *ssrfAllowListResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ssrfAllowListModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ssrfAllowListResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ssrfAllowListModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.read(ctx, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ssrfAllowListResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ssrfAllowListModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete only forgets the list. Portainer has no endpoint to remove the allow
// list, and turning a security control off is a decision to make explicitly
// rather than a side effect of no longer managing it in Terraform.
func (r *ssrfAllowListResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

func (r *ssrfAllowListResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
