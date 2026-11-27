package internal

import (
	"context"
	"fmt"
	"math"
	"reflect"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Helpers of the Kubernetes resources that are not defined by a manifest
// (batch A2 of the plugin-framework migration).
//
// Those resources build their API payloads from nested blocks several levels
// deep and read them back the same way, so instead of one model struct per
// block they work on the SDKv2-shaped view of a plan or state: a
// map[string]interface{} where a null value reads as its zero value, exactly
// as d.Get returned it. That keeps the payload code of the SDKv2 version - and
// with it the requests - unchanged. k8sfwStateValue goes the other way and
// builds the state from such a map, keeping null what the configuration left
// null.

// k8sfwGetter is a plan, state or configuration.
type k8sfwGetter interface {
	Get(ctx context.Context, target interface{}) diag.Diagnostics
}

// k8sfwObject reads a whole plan, state or configuration.
func k8sfwObject(ctx context.Context, src k8sfwGetter, diags *diag.Diagnostics) types.Object {
	var obj types.Object
	diags.Append(src.Get(ctx, &obj)...)
	return obj
}

// k8sfwData reads a whole plan, state or configuration into its SDKv2 view.
func k8sfwData(ctx context.Context, src k8sfwGetter, diags *diag.Diagnostics) (map[string]interface{}, types.Object) {
	obj := k8sfwObject(ctx, src, diags)
	data, _ := k8sfwGoValue(obj).(map[string]interface{})
	if data == nil {
		data = map[string]interface{}{}
	}
	return data, obj
}

// k8sfwGoValue converts a value to what d.Get returned for it: a null or
// unknown value is its type's zero value, numbers are int, lists and sets
// []interface{}, maps and objects map[string]interface{}.
func k8sfwGoValue(v attr.Value) interface{} {
	switch val := v.(type) {
	case basetypes.StringValue:
		return val.ValueString()
	case basetypes.Int64Value:
		return int(val.ValueInt64())
	case basetypes.BoolValue:
		return val.ValueBool()
	case basetypes.Float64Value:
		return val.ValueFloat64()
	case basetypes.NumberValue:
		if val.IsNull() || val.IsUnknown() {
			return 0
		}
		f, _ := val.ValueBigFloat().Float64()
		return f
	case basetypes.ListValue:
		return k8sfwGoList(val.Elements())
	case basetypes.SetValue:
		return k8sfwGoList(val.Elements())
	case basetypes.MapValue:
		out := map[string]interface{}{}
		for k, e := range val.Elements() {
			out[k] = k8sfwGoValue(e)
		}
		return out
	case basetypes.ObjectValue:
		out := map[string]interface{}{}
		if val.IsNull() || val.IsUnknown() {
			for k, t := range val.AttributeTypes(context.Background()) {
				out[k] = k8sfwGoValue(k8sfwNull(t))
			}
			return out
		}
		for k, e := range val.Attributes() {
			out[k] = k8sfwGoValue(e)
		}
		return out
	}
	return nil
}

// k8sfwGoList converts list elements. A block element that sets nothing
// (`block {}` without defaults) is nil, as SDKv2 returned it.
func k8sfwGoList(elems []attr.Value) []interface{} {
	out := make([]interface{}, 0, len(elems))
	for _, e := range elems {
		if obj, ok := e.(basetypes.ObjectValue); ok && k8sfwUnsetObject(obj) {
			out = append(out, nil)
			continue
		}
		out = append(out, k8sfwGoValue(e))
	}
	return out
}

// k8sfwUnsetObject reports whether every attribute of a known object is null,
// unknown or an empty collection.
func k8sfwUnsetObject(obj basetypes.ObjectValue) bool {
	if obj.IsNull() || obj.IsUnknown() {
		return false
	}
	for _, v := range obj.Attributes() {
		if v.IsNull() || v.IsUnknown() {
			continue
		}
		switch val := v.(type) {
		case basetypes.ListValue:
			if len(val.Elements()) == 0 {
				continue
			}
		case basetypes.SetValue:
			if len(val.Elements()) == 0 {
				continue
			}
		case basetypes.MapValue:
			if len(val.Elements()) == 0 {
				continue
			}
		}
		return false
	}
	return true
}

// k8sfwNull returns the null value of t.
func k8sfwNull(t attr.Type) attr.Value {
	switch tt := t.(type) {
	case basetypes.StringType:
		return types.StringNull()
	case basetypes.Int64Type:
		return types.Int64Null()
	case basetypes.BoolType:
		return types.BoolNull()
	case basetypes.Float64Type:
		return types.Float64Null()
	case basetypes.ListType:
		return types.ListNull(tt.ElemType)
	case basetypes.SetType:
		return types.SetNull(tt.ElemType)
	case basetypes.MapType:
		return types.MapNull(tt.ElemType)
	case basetypes.ObjectType:
		return types.ObjectNull(tt.AttrTypes)
	}
	v, _ := t.ValueFromTerraform(context.Background(), tftypes.NewValue(t.TerraformType(context.Background()), nil))
	return v
}

// k8sfwIsZero reports whether v is what SDKv2 stored for an unset attribute:
// an empty string, 0, false, or an empty list or map.
func k8sfwIsZero(v attr.Value) bool {
	if v.IsNull() {
		return true
	}
	switch val := v.(type) {
	case basetypes.StringValue:
		return val.ValueString() == ""
	case basetypes.Int64Value:
		return val.ValueInt64() == 0
	case basetypes.BoolValue:
		return !val.ValueBool()
	case basetypes.Float64Value:
		return val.ValueFloat64() == 0
	case basetypes.ListValue:
		return len(val.Elements()) == 0
	case basetypes.SetValue:
		return len(val.Elements()) == 0
	case basetypes.MapValue:
		return len(val.Elements()) == 0
	}
	return false
}

// k8sfwConvert builds a value of type t from a Go value as the SDKv2 flatten
// code produced it. nil is the zero value (an empty list or map).
func k8sfwConvert(t attr.Type, raw interface{}) (attr.Value, error) {
	switch tt := t.(type) {
	case basetypes.StringType:
		switch v := raw.(type) {
		case nil:
			return types.StringValue(""), nil
		case string:
			return types.StringValue(v), nil
		}
	case basetypes.Int64Type:
		switch v := raw.(type) {
		case nil:
			return types.Int64Value(0), nil
		case int:
			return types.Int64Value(int64(v)), nil
		case int32:
			return types.Int64Value(int64(v)), nil
		case int64:
			return types.Int64Value(v), nil
		case float64:
			return types.Int64Value(int64(math.Round(v))), nil
		}
	case basetypes.BoolType:
		switch v := raw.(type) {
		case nil:
			return types.BoolValue(false), nil
		case bool:
			return types.BoolValue(v), nil
		}
	case basetypes.Float64Type:
		switch v := raw.(type) {
		case nil:
			return types.Float64Value(0), nil
		case float64:
			return types.Float64Value(v), nil
		case int:
			return types.Float64Value(float64(v)), nil
		}
	case basetypes.ListType, basetypes.SetType:
		var elemType attr.Type
		if l, ok := tt.(basetypes.ListType); ok {
			elemType = l.ElemType
		} else {
			elemType = tt.(basetypes.SetType).ElemType
		}
		elems := []attr.Value{}
		if raw != nil {
			rv := reflect.ValueOf(raw)
			if rv.Kind() != reflect.Slice {
				break
			}
			for i := 0; i < rv.Len(); i++ {
				e, err := k8sfwConvert(elemType, rv.Index(i).Interface())
				if err != nil {
					return nil, err
				}
				elems = append(elems, e)
			}
		}
		if _, ok := tt.(basetypes.ListType); ok {
			v, d := types.ListValue(elemType, elems)
			if d.HasError() {
				return nil, fmt.Errorf("building a list: %v", d)
			}
			return v, nil
		}
		v, d := types.SetValue(elemType, elems)
		if d.HasError() {
			return nil, fmt.Errorf("building a set: %v", d)
		}
		return v, nil
	case basetypes.MapType:
		elems := map[string]attr.Value{}
		if raw != nil {
			rv := reflect.ValueOf(raw)
			if rv.Kind() != reflect.Map {
				break
			}
			for _, k := range rv.MapKeys() {
				e, err := k8sfwConvert(tt.ElemType, rv.MapIndex(k).Interface())
				if err != nil {
					return nil, err
				}
				elems[fmt.Sprint(k.Interface())] = e
			}
		}
		v, d := types.MapValue(tt.ElemType, elems)
		if d.HasError() {
			return nil, fmt.Errorf("building a map: %v", d)
		}
		return v, nil
	case basetypes.ObjectType:
		m, _ := raw.(map[string]interface{})
		attrs := make(map[string]attr.Value, len(tt.AttrTypes))
		for k, at := range tt.AttrTypes {
			e, err := k8sfwConvert(at, m[k])
			if err != nil {
				return nil, err
			}
			attrs[k] = e
		}
		v, d := types.ObjectValue(tt.AttrTypes, attrs)
		if d.HasError() {
			return nil, fmt.Errorf("building an object: %v", d)
		}
		return v, nil
	}
	return nil, fmt.Errorf("cannot store %T as %s", raw, t)
}

// k8sfwAttrs returns the attributes of an object value, nil when it is null,
// unknown or not an object.
func k8sfwAttrs(v attr.Value) map[string]attr.Value {
	obj, ok := v.(basetypes.ObjectValue)
	if !ok || obj.IsNull() || obj.IsUnknown() {
		return nil
	}
	return obj.Attributes()
}

// k8sfwElems returns the elements of a list value, nil when it is null,
// unknown or not a list.
func k8sfwElems(v attr.Value) []attr.Value {
	l, ok := v.(basetypes.ListValue)
	if !ok || l.IsNull() || l.IsUnknown() {
		return nil
	}
	return l.Elements()
}

// k8sfwDefault returns the schema default of an attribute, nil without one.
func k8sfwDefault(ctx context.Context, a schema.Attribute) attr.Value {
	switch at := a.(type) {
	case schema.StringAttribute:
		if at.Default != nil {
			resp := &defaults.StringResponse{}
			at.Default.DefaultString(ctx, defaults.StringRequest{}, resp)
			return resp.PlanValue
		}
	case schema.Int64Attribute:
		if at.Default != nil {
			resp := &defaults.Int64Response{}
			at.Default.DefaultInt64(ctx, defaults.Int64Request{}, resp)
			return resp.PlanValue
		}
	case schema.BoolAttribute:
		if at.Default != nil {
			resp := &defaults.BoolResponse{}
			at.Default.DefaultBool(ctx, defaults.BoolRequest{}, resp)
			return resp.PlanValue
		}
	}
	return nil
}

// k8sfwBuild builds an object of the given attributes and blocks from its
// SDKv2 view. prior is the value the object had before (the prior state in
// Read, the plan in Create and Update; null after an import):
//   - an attribute missing from data keeps its prior value, or its schema
//     default when there is none (attributes Read does not refresh);
//   - an optional, non-computed attribute whose new value is a zero value
//     stays null when it was null: SDKv2 treated null and the zero value as
//     equal, the framework does not, and the configuration left it null.
//     SDKv2 stored such an attribute inside a block as its zero value, so
//     with sdkState (a state SDKv2 wrote) a zero value counts as null too;
//   - a block element is built against the prior element at the same index.
func k8sfwBuild(ctx context.Context, attrs map[string]schema.Attribute, blocks map[string]schema.Block, data map[string]interface{}, prior attr.Value, sdkState bool) (map[string]attr.Value, error) {
	priorAttrs := k8sfwAttrs(prior)
	out := make(map[string]attr.Value, len(attrs)+len(blocks))
	for name, a := range attrs {
		t := a.GetType()
		before := priorAttrs[name]
		raw, present := data[name]
		if !present {
			switch {
			case before != nil && !before.IsUnknown() && !before.IsNull():
				out[name] = before
			case k8sfwDefault(ctx, a) != nil:
				out[name] = k8sfwDefault(ctx, a)
			default:
				out[name] = k8sfwNull(t)
			}
			continue
		}
		if v, ok := raw.(attr.Value); ok {
			out[name] = v
			continue
		}
		v, err := k8sfwConvert(t, raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if a.IsOptional() && !a.IsComputed() && k8sfwIsZero(v) && (before == nil || before.IsNull() || (sdkState && k8sfwIsZero(before))) {
			v = k8sfwNull(t)
		}
		out[name] = v
	}
	for name, b := range blocks {
		before := priorAttrs[name]
		raw, present := data[name]
		lb, isList := b.(schema.ListNestedBlock)
		if !present || !isList {
			if before != nil && !before.IsUnknown() {
				out[name] = before
			} else {
				out[name] = k8sfwNull(b.Type())
			}
			continue
		}
		elemType, _ := lb.NestedObject.Type().(basetypes.ObjectType)
		priorElems := k8sfwElems(before)
		var elems []attr.Value
		if raw != nil {
			rv := reflect.ValueOf(raw)
			if rv.Kind() != reflect.Slice {
				return nil, fmt.Errorf("%s: cannot store %T as a block list", name, raw)
			}
			for i := 0; i < rv.Len(); i++ {
				item, _ := rv.Index(i).Interface().(map[string]interface{})
				if item == nil {
					item = map[string]interface{}{}
				}
				var priorElem attr.Value
				if i < len(priorElems) {
					priorElem = priorElems[i]
				}
				built, err := k8sfwBuild(ctx, lb.NestedObject.Attributes, lb.NestedObject.Blocks, item, priorElem, sdkState)
				if err != nil {
					return nil, fmt.Errorf("%s.%d.%w", name, i, err)
				}
				obj, d := types.ObjectValue(elemType.AttrTypes, built)
				if d.HasError() {
					return nil, fmt.Errorf("%s.%d: %v", name, i, d)
				}
				elems = append(elems, obj)
			}
		}
		l, d := types.ListValue(elemType, elems)
		if d.HasError() {
			return nil, fmt.Errorf("%s: %v", name, d)
		}
		out[name] = l
	}
	return out, nil
}

// k8sfwSetState stores data, the SDKv2 view of the resource, as the new
// state (see k8sfwBuild for prior and sdkState).
func k8sfwSetState(ctx context.Context, s schema.Schema, state *tfsdk.State, data map[string]interface{}, prior attr.Value, sdkState bool, diags *diag.Diagnostics) {
	built, err := k8sfwBuild(ctx, s.Attributes, s.Blocks, data, prior, sdkState)
	if err != nil {
		diags.AddError("Failed to build the state", err.Error())
		return
	}
	objType, _ := s.Type().(basetypes.ObjectType)
	obj, d := types.ObjectValue(objType.AttrTypes, built)
	diags.Append(d...)
	if diags.HasError() {
		return
	}
	diags.Append(state.Set(ctx, obj)...)
}

// k8sfwStateKey marks, in private state, a state this provider wrote with
// the plugin framework. A state without it was written by SDKv2 (v3.0.0 and
// before), or comes from an import.
const k8sfwStateKey = "framework_state"

// k8sfwPrivateReader is the private state of a request.
type k8sfwPrivateReader interface {
	GetKey(ctx context.Context, key string) ([]byte, diag.Diagnostics)
}

// k8sfwPrivateWriter is the private state of a response.
type k8sfwPrivateWriter interface {
	SetKey(ctx context.Context, key string, value []byte) diag.Diagnostics
}

// k8sfwFromSDK reports whether the prior state was written by SDKv2.
func k8sfwFromSDK(ctx context.Context, private k8sfwPrivateReader, diags *diag.Diagnostics) bool {
	v, d := private.GetKey(ctx, k8sfwStateKey)
	diags.Append(d...)
	return len(v) == 0
}

// k8sfwMark marks the new state as written by the framework.
func k8sfwMark(ctx context.Context, private k8sfwPrivateWriter, diags *diag.Diagnostics) {
	diags.Append(private.SetKey(ctx, k8sfwStateKey, []byte(`true`))...)
}

// k8sfwSchemaOf returns the schema of a framework resource.
func k8sfwSchemaOf(ctx context.Context, r resource.Resource) schema.Schema {
	resp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, resp)
	return resp.Schema
}

// k8sfwGetOk is d.GetOk on the SDKv2 view: the value and whether it is set
// to something other than its zero value.
func k8sfwGetOk(data map[string]interface{}, key string) (interface{}, bool) {
	v, ok := data[key]
	if !ok || v == nil {
		return v, false
	}
	switch val := v.(type) {
	case string:
		return v, val != ""
	case int:
		return v, val != 0
	case bool:
		return v, val
	case []interface{}:
		return v, len(val) > 0
	case map[string]interface{}:
		return v, len(val) > 0
	}
	return v, true
}

// k8sfwList returns data[key] as a list, empty when unset.
func k8sfwList(data map[string]interface{}, key string) []interface{} {
	l, _ := data[key].([]interface{})
	return l
}

// k8sfwMap returns data[key] as a map, empty when unset.
func k8sfwMap(data map[string]interface{}, key string) map[string]interface{} {
	m, _ := data[key].(map[string]interface{})
	if m == nil {
		m = map[string]interface{}{}
	}
	return m
}

// k8sfwStrings returns a list of strings from the SDKv2 view.
func k8sfwStrings(raw interface{}) []string {
	l, _ := raw.([]interface{})
	out := make([]string, 0, len(l))
	for _, v := range l {
		s, _ := v.(string)
		out = append(out, s)
	}
	return out
}

// k8sfwStringMap returns a map of strings from the SDKv2 view.
func k8sfwStringMap(raw interface{}) map[string]string {
	m, _ := raw.(map[string]interface{})
	out := make(map[string]string, len(m))
	for k, v := range m {
		s, _ := v.(string)
		out[k] = s
	}
	return out
}

// k8sfwBase is embedded by the resources of this batch: it holds the client
// the provider configured.
type k8sfwBase struct{ client *APIClient }

func (r *k8sfwBase) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

// k8sfwKeepState is the Update of a resource whose every argument forces a
// replacement (only "timeouts" can change in place): the plan becomes the
// state, computed values staying what they were.
func k8sfwKeepState(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	plan := k8sfwObject(ctx, req.Plan, &resp.Diagnostics)
	state := k8sfwObject(ctx, req.State, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	attrs := plan.Attributes()
	before := state.Attributes()
	merged := make(map[string]attr.Value, len(attrs))
	for k, v := range attrs {
		if v.IsUnknown() {
			if s, ok := before[k]; ok {
				v = s
			}
		}
		merged[k] = v
	}
	obj, d := types.ObjectValue(plan.AttributeTypes(ctx), merged)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, obj)...)
}

// k8sfwImportDefaults stores the schema default of every top-level attribute
// that has one. SDKv2 read an attribute missing from an imported state as its
// zero value, which is what these defaults are, so an argument Read does not
// refresh plans no change (or, when it forces a new resource, no
// replacement) after an import, as before.
func k8sfwImportDefaults(ctx context.Context, s schema.Schema, state *tfsdk.State, diags *diag.Diagnostics) {
	for name, a := range s.Attributes {
		if v := k8sfwDefault(ctx, a); v != nil {
			diags.Append(state.SetAttribute(ctx, path.Root(name), v)...)
		}
	}
}

// k8sfwFillUnknowns returns plan with every unknown value replaced by the
// value at the same place in read (null when read has none). After Create and
// Update the framework requires the state to hold the planned value wherever
// the plan knew it; SDKv2 let a read right after the apply overwrite those.
func k8sfwFillUnknowns(plan, read attr.Value) attr.Value {
	if plan.IsUnknown() {
		if read == nil || read.IsUnknown() {
			return k8sfwNull(plan.Type(context.Background()))
		}
		return read
	}
	if plan.IsNull() {
		return plan
	}
	switch p := plan.(type) {
	case basetypes.ObjectValue:
		readAttrs := k8sfwAttrs(read)
		attrs := make(map[string]attr.Value, len(p.Attributes()))
		for k, v := range p.Attributes() {
			attrs[k] = k8sfwFillUnknowns(v, readAttrs[k])
		}
		obj, d := types.ObjectValue(p.AttributeTypes(context.Background()), attrs)
		if d.HasError() {
			return plan
		}
		return obj
	case basetypes.ListValue:
		readElems := k8sfwElems(read)
		elems := make([]attr.Value, 0, len(p.Elements()))
		for i, v := range p.Elements() {
			var r attr.Value
			if i < len(readElems) {
				r = readElems[i]
			}
			elems = append(elems, k8sfwFillUnknowns(v, r))
		}
		l, d := types.ListValue(p.ElementType(context.Background()), elems)
		if d.HasError() {
			return plan
		}
		return l
	}
	return plan
}

// k8sfwSetApplied stores the result of a Create or Update: data, the SDKv2
// view as read back after the apply, fills in what the plan left unknown.
func k8sfwSetApplied(ctx context.Context, s schema.Schema, state *tfsdk.State, data map[string]interface{}, plan types.Object, diags *diag.Diagnostics) {
	built, err := k8sfwBuild(ctx, s.Attributes, s.Blocks, data, plan, false)
	if err != nil {
		diags.AddError("Failed to build the state", err.Error())
		return
	}
	objType, _ := s.Type().(basetypes.ObjectType)
	read, d := types.ObjectValue(objType.AttrTypes, built)
	diags.Append(d...)
	if diags.HasError() {
		return
	}
	diags.Append(state.Set(ctx, k8sfwFillUnknowns(plan, read))...)
}

// k8sfwKeepNull keeps an attribute with a default null in the plan when the
// configuration leaves it out and the state holds null: SDKv2 stored null
// for an attribute left at a default that is its zero value ("", 0, false),
// so the framework planned the default as a change - a replacement where it
// forces one - on the first plan after upgrading from v3.0.0.
type k8sfwKeepNull struct{}

func (k8sfwKeepNull) Description(context.Context) string {
	return "Keeps null in the plan when the configuration and the state leave the attribute unset."
}

func (m k8sfwKeepNull) MarkdownDescription(ctx context.Context) string { return m.Description(ctx) }

func (k8sfwKeepNull) keep(state tfsdk.State, configNull, stateNull bool) bool {
	return !state.Raw.IsNull() && configNull && stateNull
}

func (m k8sfwKeepNull) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if m.keep(req.State, req.ConfigValue.IsNull(), req.StateValue.IsNull()) {
		resp.PlanValue = req.StateValue
	}
}

func (m k8sfwKeepNull) PlanModifyInt64(_ context.Context, req planmodifier.Int64Request, resp *planmodifier.Int64Response) {
	if m.keep(req.State, req.ConfigValue.IsNull(), req.StateValue.IsNull()) {
		resp.PlanValue = req.StateValue
	}
}

func (m k8sfwKeepNull) PlanModifyBool(_ context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	if m.keep(req.State, req.ConfigValue.IsNull(), req.StateValue.IsNull()) {
		resp.PlanValue = req.StateValue
	}
}
