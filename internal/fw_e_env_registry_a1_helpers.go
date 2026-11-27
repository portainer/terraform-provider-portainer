package internal

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Helpers of the framework endpoint group, endpoint group access, membership,
// relations and association resources.
//
// SDKv2 treated "", 0 and [] as equal to null. v3.0.0 left an omitted list
// null in the state, but stored "" or 0 for an omitted string or number it
// set (endpoint_group.description, endpoint_relations.relation.group_id). A
// zero value coming back from the API is written as null when the prior
// value was null; egKeepZeroString/egKeepZeroInt64 keep the "" or 0 of a
// v3.0.0 state when the configuration omits the attribute, so upgrading
// plans no change.

// egOptString is the state value of an optional (non-computed) string read
// from the API: null when the API returns "" and the prior value was null,
// otherwise the API value.
func egOptString(api string, prior types.String) types.String {
	if api == "" && prior.IsNull() {
		return types.StringNull()
	}
	return types.StringValue(api)
}

// egOptInt64List is the state value of an optional (non-computed) list(number)
// read from the API: null when the API returns no element and the prior
// value was null, otherwise the API values.
func egOptInt64List(api []int64, prior types.List) types.List {
	if len(api) == 0 && prior.IsNull() {
		return types.ListNull(types.Int64Type)
	}
	ints := make([]int, 0, len(api))
	for _, v := range api {
		ints = append(ints, int(v))
	}
	return int64ListValue(ints)
}

// egInt64Set reports whether an optional number is set to a non-zero value,
// the way SDKv2's d.GetOk saw it.
func egInt64Set(v types.Int64) bool {
	return !v.IsNull() && !v.IsUnknown() && v.ValueInt64() != 0
}

// egInt64Slice reads a list(number) into []int64 (nil for null, unknown or
// empty, the way SDKv2's d.GetOk saw an empty list).
func egInt64Slice(l types.List) []int64 {
	if l.IsNull() || l.IsUnknown() {
		return nil
	}
	var out []int64
	for _, e := range l.Elements() {
		if n, ok := e.(types.Int64); ok && !n.IsNull() && !n.IsUnknown() {
			out = append(out, n.ValueInt64())
		}
	}
	return out
}

// egRequiresReplaceUnlessZero is RequiresReplace for an optional number
// whose 0 means "not set" (SDKv2 could not tell them apart): moving between
// null and 0 does not replace the resource.
func egRequiresReplaceUnlessZero() planmodifier.Int64 {
	return int64planmodifier.RequiresReplaceIf(
		func(_ context.Context, req planmodifier.Int64Request, resp *int64planmodifier.RequiresReplaceIfFuncResponse) {
			resp.RequiresReplace = egInt64Set(req.StateValue) || egInt64Set(req.PlanValue)
		},
		"Changing this value forces resource recreation.",
		"Changing this value forces resource recreation.",
	)
}

// egKeepZeroString plans an omitted optional+computed string as null, or as
// the prior "" a v3.0.0 state holds, which SDKv2 treated as the same thing.
// Optional+computed only so that "" can stay: the value is never taken from
// the API when the configuration omits it, as before.
type egKeepZeroString struct{}

func (egKeepZeroString) Description(context.Context) string {
	return "An omitted value is null, or the empty string of a state written by v3.0.0."
}

func (m egKeepZeroString) MarkdownDescription(ctx context.Context) string { return m.Description(ctx) }

func (egKeepZeroString) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if !req.ConfigValue.IsNull() {
		return
	}
	if !req.StateValue.IsNull() && !req.StateValue.IsUnknown() && req.StateValue.ValueString() == "" {
		resp.PlanValue = req.StateValue
		return
	}
	resp.PlanValue = types.StringNull()
}

// egKeepZeroInt64 is egKeepZeroString for a number and its 0.
type egKeepZeroInt64 struct{}

func (egKeepZeroInt64) Description(context.Context) string {
	return "An omitted value is null, or the 0 of a state written by v3.0.0."
}

func (m egKeepZeroInt64) MarkdownDescription(ctx context.Context) string { return m.Description(ctx) }

func (egKeepZeroInt64) PlanModifyInt64(_ context.Context, req planmodifier.Int64Request, resp *planmodifier.Int64Response) {
	if !req.ConfigValue.IsNull() {
		return
	}
	if !req.StateValue.IsNull() && !req.StateValue.IsUnknown() && req.StateValue.ValueInt64() == 0 {
		resp.PlanValue = req.StateValue
		return
	}
	resp.PlanValue = types.Int64Null()
}
