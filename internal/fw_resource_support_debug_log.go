package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*supportDebugLogResource)(nil)
	_ resource.ResourceWithConfigure   = (*supportDebugLogResource)(nil)
	_ resource.ResourceWithImportState = (*supportDebugLogResource)(nil)
)

func newSupportDebugLogResource() resource.Resource { return &supportDebugLogResource{} }

type supportDebugLogResource struct{ client *APIClient }

type supportDebugLogModel struct {
	ID      types.String `tfsdk:"id"`
	Enabled types.Bool   `tfsdk:"enabled"`
}

func (r *supportDebugLogResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_support_debug_log"
}

func (r *supportDebugLogResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			// The ID is the enabled flag, so it changes with it.
			"id":      schema.StringAttribute{Description: "Identifier of the resource.", Computed: true},
			"enabled": schema.BoolAttribute{Required: true, Description: "Enable or disable the global debug log"},
		},
	}
}

func (r *supportDebugLogResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *supportDebugLogResource) apply(ctx context.Context, enabled bool) error {
	payload := map[string]bool{"debugLogEnabled": enabled}
	if err := doJSON(ctx, r.client, http.MethodPut, fmt.Sprintf("%s/support/debug_log", r.client.Endpoint), payload, nil); err != nil {
		return fmt.Errorf("failed to set debug log: %w", err)
	}
	return nil
}

func (r *supportDebugLogResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan supportDebugLogModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.apply(ctx, plan.Enabled.ValueBool()); err != nil {
		resp.Diagnostics.AddError("Failed to set debug log", err.Error())
		return
	}
	plan.ID = types.StringValue(strconv.FormatBool(plan.Enabled.ValueBool()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *supportDebugLogResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan supportDebugLogModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.apply(ctx, plan.Enabled.ValueBool()); err != nil {
		resp.Diagnostics.AddError("Failed to set debug log", err.Error())
		return
	}
	plan.ID = types.StringValue(strconv.FormatBool(plan.Enabled.ValueBool()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *supportDebugLogResource) Read(ctx context.Context, _ resource.ReadRequest, resp *resource.ReadResponse) {
	var result struct {
		DebugLogEnabled bool `json:"debugLogEnabled"`
	}
	if err := doJSON(ctx, r.client, http.MethodGet, fmt.Sprintf("%s/support/debug_log", r.client.Endpoint), nil, &result); err != nil {
		resp.Diagnostics.AddError("Failed to read debug log status", fmt.Sprintf("failed to read debug log status: %s", err))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("enabled"), result.DebugLogEnabled)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), strconv.FormatBool(result.DebugLogEnabled))...)
}

// Delete switches the debug log off.
func (r *supportDebugLogResource) Delete(ctx context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	if err := r.apply(ctx, false); err != nil {
		resp.Diagnostics.AddError("Failed to set debug log", err.Error())
	}
}

func (r *supportDebugLogResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
