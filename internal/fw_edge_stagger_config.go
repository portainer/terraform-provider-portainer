package internal

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// The stagger_config block of portainer_edge_stack and of the edge_settings
// of portainer_custom_template: Portainer's EdgeStaggerConfig, the Business
// Edition setting that rolls an Edge stack update out to the devices in
// batches instead of all at once. Its enums are numbers in the API and names
// here, like the rest of the provider's newer blocks.

// edgeStaggerAttrTypes is the object type of a stagger_config block.
var edgeStaggerAttrTypes = map[string]attr.Type{
	"stagger_option":             types.StringType,
	"parallel_option":            types.StringType,
	"device_number":              types.Int64Type,
	"device_number_start_from":   types.Int64Type,
	"device_number_increment_by": types.Int64Type,
	"timeout":                    types.StringType,
	"update_delay":               types.StringType,
	"update_failure_action":      types.StringType,
}

type edgeStaggerModel struct {
	StaggerOption           types.String `tfsdk:"stagger_option"`
	ParallelOption          types.String `tfsdk:"parallel_option"`
	DeviceNumber            types.Int64  `tfsdk:"device_number"`
	DeviceNumberStartFrom   types.Int64  `tfsdk:"device_number_start_from"`
	DeviceNumberIncrementBy types.Int64  `tfsdk:"device_number_increment_by"`
	Timeout                 types.String `tfsdk:"timeout"`
	UpdateDelay             types.String `tfsdk:"update_delay"`
	UpdateFailureAction     types.String `tfsdk:"update_failure_action"`
}

// edgeStaggerAPI is EdgeStaggerConfig as Portainer reports it.
type edgeStaggerAPI struct {
	StaggerOption           int    `json:"StaggerOption"`
	StaggerParallelOption   int    `json:"StaggerParallelOption"`
	DeviceNumber            int64  `json:"DeviceNumber"`
	DeviceNumberStartFrom   int64  `json:"DeviceNumberStartFrom"`
	DeviceNumberIncrementBy int64  `json:"DeviceNumberIncrementBy"`
	Timeout                 string `json:"Timeout"`
	UpdateDelay             string `json:"UpdateDelay"`
	UpdateFailureAction     int    `json:"UpdateFailureAction"`
}

// The API values of the enums; index 0 is Portainer's "unset".
var (
	edgeStaggerOptions        = []string{"", "all_at_once", "parallel"}
	edgeStaggerParallelOpts   = []string{"", "fixed", "incremental"}
	edgeStaggerFailureActions = []string{"", "continue", "pause", "rollback"}
)

func edgeEnumIndex(names []string, name string) int {
	for i, n := range names {
		if i > 0 && n == name {
			return i
		}
	}
	return 0
}

func edgeEnumName(names []string, v int) string {
	if v > 0 && v < len(names) {
		return names[v]
	}
	return ""
}

// edgeStaggerConfigBlock is the stagger_config block schema.
func edgeStaggerConfigBlock() schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description: "Staggered rollout of the Edge stack to its devices. Business Edition only; Community Edition ignores it. Only sent when the block is set, so leaving it out keeps what Portainer has recorded.",
		Validators:  []validator.List{listvalidator.SizeAtMost(1)},
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"stagger_option": schema.StringAttribute{
					Required:    true,
					Validators:  []validator.String{stringvalidator.OneOf(edgeStaggerOptions[1:]...)},
					Description: "How devices are updated: `all_at_once` or `parallel` (in batches).",
				},
				"parallel_option": schema.StringAttribute{
					Optional:    true,
					Validators:  []validator.String{stringvalidator.OneOf(edgeStaggerParallelOpts[1:]...)},
					Description: "Batch size policy for `parallel`: `fixed` (always `device_number` devices) or `incremental` (growing by `device_number_increment_by`).",
				},
				"device_number": schema.Int64Attribute{
					Optional:    true,
					Validators:  []validator.Int64{int64validator.AtLeast(0)},
					Description: "Number of devices updated at once with the `fixed` parallel option.",
				},
				"device_number_start_from": schema.Int64Attribute{
					Optional:    true,
					Validators:  []validator.Int64{int64validator.AtLeast(0)},
					Description: "Size of the first batch with the `incremental` parallel option.",
				},
				"device_number_increment_by": schema.Int64Attribute{
					Optional:    true,
					Validators:  []validator.Int64{int64validator.AtLeast(0)},
					Description: "Factor by which each batch grows with the `incremental` parallel option.",
				},
				"timeout": schema.StringAttribute{
					Optional:    true,
					Description: "Minutes a batch may take before it counts as failed, as a string (for example `\"5\"`).",
				},
				"update_delay": schema.StringAttribute{
					Optional:    true,
					Description: "Minutes to wait between batches, as a string (for example `\"5\"`).",
				},
				"update_failure_action": schema.StringAttribute{
					Optional:    true,
					Validators:  []validator.String{stringvalidator.OneOf(edgeStaggerFailureActions[1:]...)},
					Description: "What happens when a batch fails: `continue`, `pause` or `rollback`.",
				},
			},
		},
	}
}

// edgeStaggerPayload returns the EdgeStaggerConfig payload of a
// stagger_config block, or nil when the block is not set: then nothing is
// sent and Portainer keeps its recorded configuration (or its default).
func edgeStaggerPayload(ctx context.Context, l types.List, diags *diag.Diagnostics) map[string]interface{} {
	if l.IsNull() || l.IsUnknown() || len(l.Elements()) == 0 {
		return nil
	}
	var blocks []edgeStaggerModel
	diags.Append(l.ElementsAs(ctx, &blocks, false)...)
	if len(blocks) == 0 {
		return nil
	}
	b := blocks[0]
	payload := map[string]interface{}{"StaggerOption": edgeEnumIndex(edgeStaggerOptions, b.StaggerOption.ValueString())}
	if v := edgeEnumIndex(edgeStaggerParallelOpts, b.ParallelOption.ValueString()); v != 0 {
		payload["StaggerParallelOption"] = v
	}
	if v := edgeEnumIndex(edgeStaggerFailureActions, b.UpdateFailureAction.ValueString()); v != 0 {
		payload["UpdateFailureAction"] = v
	}
	for key, v := range map[string]types.Int64{
		"DeviceNumber": b.DeviceNumber, "DeviceNumberStartFrom": b.DeviceNumberStartFrom, "DeviceNumberIncrementBy": b.DeviceNumberIncrementBy,
	} {
		if !v.IsNull() && !v.IsUnknown() {
			payload[key] = v.ValueInt64()
		}
	}
	for key, v := range map[string]types.String{"Timeout": b.Timeout, "UpdateDelay": b.UpdateDelay} {
		if v.ValueString() != "" {
			payload[key] = v.ValueString()
		}
	}
	return payload
}

// edgeStaggerFromAPI reads a stagger_config block back. Only a block the
// configuration manages is read: Portainer records a default stagger
// configuration for every stack, which would otherwise show as a block to
// remove on every plan. A server that reports none (Community Edition) leaves
// the block as it was. Unset attributes stay null while Portainer reports
// their zero value.
func edgeStaggerFromAPI(ctx context.Context, prior types.List, api *edgeStaggerAPI, diags *diag.Diagnostics) types.List {
	objType := types.ObjectType{AttrTypes: edgeStaggerAttrTypes}
	if prior.IsNull() || prior.IsUnknown() || len(prior.Elements()) == 0 {
		return types.ListValueMust(objType, []attr.Value{})
	}
	if api == nil {
		return prior
	}
	var blocks []edgeStaggerModel
	diags.Append(prior.ElementsAs(ctx, &blocks, false)...)
	if len(blocks) == 0 {
		return prior
	}
	p := blocks[0]
	option := types.StringValue(edgeEnumName(edgeStaggerOptions, api.StaggerOption))
	if api.StaggerOption == 0 {
		option = p.StaggerOption
	}
	obj, d := types.ObjectValue(edgeStaggerAttrTypes, map[string]attr.Value{
		"stagger_option":             option,
		"parallel_option":            edgeStringFromAPI(p.ParallelOption, edgeEnumName(edgeStaggerParallelOpts, api.StaggerParallelOption)),
		"device_number":              edgeInt64FromAPI(p.DeviceNumber, api.DeviceNumber),
		"device_number_start_from":   edgeInt64FromAPI(p.DeviceNumberStartFrom, api.DeviceNumberStartFrom),
		"device_number_increment_by": edgeInt64FromAPI(p.DeviceNumberIncrementBy, api.DeviceNumberIncrementBy),
		"timeout":                    edgeStringFromAPI(p.Timeout, api.Timeout),
		"update_delay":               edgeStringFromAPI(p.UpdateDelay, api.UpdateDelay),
		"update_failure_action":      edgeStringFromAPI(p.UpdateFailureAction, edgeEnumName(edgeStaggerFailureActions, api.UpdateFailureAction)),
	})
	diags.Append(d...)
	return types.ListValueMust(objType, []attr.Value{obj})
}
