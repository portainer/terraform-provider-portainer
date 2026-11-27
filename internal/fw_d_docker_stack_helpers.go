package internal

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"sort"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// The Docker and stack resources of this provider (portainer_stack,
// portainer_docker_*, ...) were written against SDKv2's ResourceData, and
// their behaviour - which attribute is read when, what Read refreshes and what
// it leaves alone - is what existing configurations and states depend on.
// They are served by the plugin framework through dkResource and
// dkDataSource below, which hand the CRUD functions a dkBag: the attribute
// values of the plan, state or configuration as plain Go values (string, int,
// bool, []interface{}, map[string]interface{}), read and written with the
// zero-value semantics those functions were written for.
//
// What the framework is strict about and SDKv2 was not is handled here, once:
//   - Create and Update return the planned value of every attribute the
//     configuration owns; only computed attributes the plan left unknown take
//     the value the CRUD function set (or null).
//   - SDKv2 stored "", 0, false or an empty collection for an optional
//     attribute the configuration leaves out, and treated it as equal to
//     null. Read writes null for such a zero value when the prior state had
//     null there, or when the state was written by SDKv2 (no dkPrivateKey in
//     the private state): otherwise the first plan after upgrading would show
//     `"" -> null`.
//   - ForceNew attributes are replaced only for a real change, not for "" vs
//     null (dkReplace*).

// dkPrivateKey marks a state written by this half of the provider, in which
// null and zero values are exact.
const dkPrivateKey = "dk_framework_state"

// dkDefaultTimeout is the timeout SDKv2 gave every operation of a resource
// that did not declare one.
const dkDefaultTimeout = 20 * time.Minute

// ---------------------------------------------------------------------------
// Schema information
// ---------------------------------------------------------------------------

// dkField is what the bag needs to know about an attribute or block.
type dkField struct {
	optional, computed, required, writeOnly bool
	block                                   bool
	nested                                  map[string]*dkField
}

type dkAttrFlags interface {
	IsOptional() bool
	IsComputed() bool
	IsRequired() bool
	IsWriteOnly() bool
}

func dkFieldOf(a dkAttrFlags) *dkField {
	return &dkField{optional: a.IsOptional(), computed: a.IsComputed(), required: a.IsRequired(), writeOnly: a.IsWriteOnly()}
}

func dkResourceFields(attrs map[string]schema.Attribute, blocks map[string]schema.Block) map[string]*dkField {
	out := map[string]*dkField{}
	for name, a := range attrs {
		out[name] = dkFieldOf(a)
	}
	for name, b := range blocks {
		f := &dkField{optional: true, block: true}
		switch bt := b.(type) {
		case schema.ListNestedBlock:
			f.nested = dkResourceFields(bt.NestedObject.Attributes, bt.NestedObject.Blocks)
		case schema.SetNestedBlock:
			f.nested = dkResourceFields(bt.NestedObject.Attributes, bt.NestedObject.Blocks)
		default:
			// timeouts: a single nested block the bag carries through as is.
			f.block = false
			f.computed = true
		}
		out[name] = f
	}
	return out
}

func dkDataSourceFields(attrs map[string]dsschema.Attribute) map[string]*dkField {
	out := map[string]*dkField{}
	for name, a := range attrs {
		out[name] = dkFieldOf(a)
	}
	return out
}

// ---------------------------------------------------------------------------
// The bag
// ---------------------------------------------------------------------------

// dkBag holds a resource's attributes the way SDKv2's ResourceData presented
// them to CRUD functions.
type dkBag struct {
	fields  map[string]*dkField
	types   map[string]tftypes.Type
	id      string
	vals    map[string]interface{} // current values (plan, or state in Read/Delete)
	old     map[string]interface{} // prior state values (HasChange)
	unknown map[string]bool        // unknown in the plan
	set     map[string]bool        // written by the CRUD function
	config  map[string]tftypes.Value
	raw     map[string]tftypes.Value // the values vals was built from
}

func newDkBag(fields map[string]*dkField, objType tftypes.Type) *dkBag {
	b := &dkBag{
		fields:  fields,
		types:   map[string]tftypes.Type{},
		vals:    map[string]interface{}{},
		old:     map[string]interface{}{},
		unknown: map[string]bool{},
		set:     map[string]bool{},
		config:  map[string]tftypes.Value{},
		raw:     map[string]tftypes.Value{},
	}
	if obj, ok := objType.(tftypes.Object); ok {
		for name, t := range obj.AttributeTypes {
			b.types[name] = t
		}
	}
	return b
}

// dkObjectAttrs splits an object value into its attributes (nil for null).
func dkObjectAttrs(v tftypes.Value) map[string]tftypes.Value {
	if v.Type() == nil || v.IsNull() || !v.IsKnown() {
		return nil
	}
	// As hands out the value's own map; callers get a copy they may change.
	var attrs map[string]tftypes.Value
	if err := v.As(&attrs); err != nil {
		return nil
	}
	out := make(map[string]tftypes.Value, len(attrs))
	for k, a := range attrs {
		out[k] = a
	}
	return out
}

// loadValues fills vals from v (a plan or state object); state also fills old.
func (b *dkBag) loadValues(v tftypes.Value, alsoOld bool) {
	attrs := dkObjectAttrs(v)
	for name, t := range b.types {
		av, ok := attrs[name]
		if !ok {
			av = tftypes.NewValue(t, nil)
		}
		b.raw[name] = av
		if !av.IsKnown() {
			b.unknown[name] = true
		}
		if name == "id" {
			if av.IsKnown() && !av.IsNull() {
				_ = av.As(&b.id)
			}
			continue
		}
		b.vals[name] = dkFromTF(av)
		if alsoOld {
			b.old[name] = dkFromTF(av)
		}
	}
}

func (b *dkBag) loadOld(v tftypes.Value) {
	attrs := dkObjectAttrs(v)
	for name := range b.types {
		if av, ok := attrs[name]; ok {
			b.old[name] = dkFromTF(av)
			if name == "id" && b.id == "" && av.IsKnown() && !av.IsNull() {
				_ = av.As(&b.id)
			}
		}
	}
}

func (b *dkBag) loadConfig(v tftypes.Value) {
	for name, av := range dkObjectAttrs(v) {
		b.config[name] = av
	}
}

func (b *dkBag) ID() string      { return b.id }
func (b *dkBag) SetID(id string) { b.id = id }

// Get returns the value of a top-level attribute, or its zero value.
func (b *dkBag) Get(key string) interface{} {
	if v, ok := b.vals[key]; ok && v != nil {
		return v
	}
	return dkZero(b.types[key])
}

// GetOk is ResourceData.GetOk: the value, and whether it is set to a non-zero
// value.
func (b *dkBag) GetOk(key string) (interface{}, bool) {
	v := b.Get(key)
	return v, !dkIsZeroGo(v)
}

// Set stores a value for a top-level attribute.
func (b *dkBag) Set(key string, value interface{}) error {
	t, ok := b.types[key]
	if !ok {
		return fmt.Errorf("invalid address to set: []string{%q}", key)
	}
	norm, err := dkNormalize(value, t)
	if err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	b.vals[key] = norm
	b.set[key] = true
	return nil
}

// SetFields is setFields for a bag.
func (b *dkBag) SetFields(fields map[string]interface{}) error {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := b.Set(k, fields[k]); err != nil {
			return err
		}
	}
	return nil
}

// HasChange reports whether the attribute differs between state and plan.
func (b *dkBag) HasChange(key string) bool {
	oldV, ok := b.old[key]
	if !ok || oldV == nil {
		oldV = dkZero(b.types[key])
	}
	return !reflect.DeepEqual(oldV, b.Get(key))
}

// ConfigValue returns the raw configuration value of a top-level attribute
// (null when absent, or when the bag was not built with a configuration).
func (b *dkBag) ConfigValue(key string) tftypes.Value {
	if v, ok := b.config[key]; ok {
		return v
	}
	if t, ok := b.types[key]; ok {
		return tftypes.NewValue(t, nil)
	}
	return tftypes.NewValue(tftypes.String, nil)
}

// Configured reports whether the configuration sets the attribute.
func (b *dkBag) Configured(key string) bool {
	return !b.ConfigValue(key).IsNull()
}

// ConfigString returns a configured string (write-only attributes included),
// "" when unset or unknown.
func (b *dkBag) ConfigString(key string) string {
	v := b.ConfigValue(key)
	if v.IsNull() || !v.IsKnown() {
		return ""
	}
	var s string
	_ = v.As(&s)
	return s
}

// ConfigBool returns a configured bool, nil when unset.
func (b *dkBag) ConfigBool(key string) *bool {
	v := b.ConfigValue(key)
	if v.IsNull() || !v.IsKnown() {
		return nil
	}
	var out bool
	if err := v.As(&out); err != nil {
		return nil
	}
	return &out
}

// Timeout returns the configured timeout of op ("create", "update",
// "delete", "read"), or def.
func (b *dkBag) Timeout(op string, def time.Duration) time.Duration {
	if m, ok := b.vals["timeouts"].(map[string]interface{}); ok {
		if s, ok := m[op].(string); ok && s != "" {
			if d, err := time.ParseDuration(s); err == nil {
				return d
			}
		}
	}
	return def
}

// ---------------------------------------------------------------------------
// Value conversion
// ---------------------------------------------------------------------------

// dkFromTF converts a tftypes value into the Go value ResourceData.Get
// returned for it. Null and unknown give the zero value.
func dkFromTF(v tftypes.Value) interface{} {
	t := v.Type()
	if v.IsNull() || !v.IsKnown() {
		return dkZero(t)
	}
	switch {
	case t.Is(tftypes.String):
		var s string
		_ = v.As(&s)
		return s
	case t.Is(tftypes.Number):
		var f big.Float
		_ = v.As(&f)
		i, _ := f.Int64()
		return int(i)
	case t.Is(tftypes.Bool):
		var bv bool
		_ = v.As(&bv)
		return bv
	case t.Is(tftypes.List{}), t.Is(tftypes.Set{}), t.Is(tftypes.Tuple{}):
		var elems []tftypes.Value
		_ = v.As(&elems)
		out := make([]interface{}, 0, len(elems))
		for _, e := range elems {
			out = append(out, dkFromTF(e))
		}
		return out
	case t.Is(tftypes.Map{}):
		var elems map[string]tftypes.Value
		_ = v.As(&elems)
		out := make(map[string]interface{}, len(elems))
		for k, e := range elems {
			out[k] = dkFromTF(e)
		}
		return out
	case t.Is(tftypes.Object{}):
		var attrs map[string]tftypes.Value
		_ = v.As(&attrs)
		out := make(map[string]interface{}, len(attrs))
		for k, e := range attrs {
			out[k] = dkFromTF(e)
		}
		return out
	}
	return nil
}

// dkZero is the zero value ResourceData.Get returned for an unset attribute.
func dkZero(t tftypes.Type) interface{} {
	switch {
	case t == nil:
		return nil
	case t.Is(tftypes.String):
		return ""
	case t.Is(tftypes.Number):
		return 0
	case t.Is(tftypes.Bool):
		return false
	case t.Is(tftypes.List{}), t.Is(tftypes.Set{}), t.Is(tftypes.Tuple{}):
		return []interface{}{}
	case t.Is(tftypes.Map{}):
		return map[string]interface{}{}
	}
	return nil
}

func dkIsZeroGo(v interface{}) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return x == ""
	case int:
		return x == 0
	case bool:
		return !x
	case []interface{}:
		return len(x) == 0
	case map[string]interface{}:
		return len(x) == 0
	}
	return false
}

// dkNormalize converts a value a CRUD function stores into the bag's
// representation of type t.
func dkNormalize(value interface{}, t tftypes.Type) (interface{}, error) {
	tv, err := dkToTF(value, t)
	if err != nil {
		return nil, err
	}
	return dkFromTF(tv), nil
}

// dkToTF converts a Go value into a tftypes value of type t. nil is null.
func dkToTF(value interface{}, t tftypes.Type) (tftypes.Value, error) {
	if value == nil {
		return tftypes.NewValue(t, nil), nil
	}
	if tv, ok := value.(tftypes.Value); ok {
		return tv, nil
	}
	rv := reflect.ValueOf(value)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return tftypes.NewValue(t, nil), nil
		}
		rv = rv.Elem()
		value = rv.Interface()
	}
	switch {
	case t.Is(tftypes.String):
		if s, ok := value.(string); ok {
			return tftypes.NewValue(t, s), nil
		}
		return tftypes.NewValue(t, fmt.Sprint(value)), nil
	case t.Is(tftypes.Number):
		switch rv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return tftypes.NewValue(t, new(big.Float).SetInt64(rv.Int())), nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return tftypes.NewValue(t, new(big.Float).SetUint64(rv.Uint())), nil
		case reflect.Float32, reflect.Float64:
			return tftypes.NewValue(t, big.NewFloat(rv.Float())), nil
		}
		return tftypes.Value{}, fmt.Errorf("'%v' expected type 'int', got unconvertible type '%T'", value, value)
	case t.Is(tftypes.Bool):
		if bv, ok := value.(bool); ok {
			return tftypes.NewValue(t, bv), nil
		}
		return tftypes.Value{}, fmt.Errorf("'%v' expected type 'bool', got unconvertible type '%T'", value, value)
	case t.Is(tftypes.List{}), t.Is(tftypes.Set{}):
		var et tftypes.Type
		if lt, ok := t.(tftypes.List); ok {
			et = lt.ElementType
		} else {
			et = t.(tftypes.Set).ElementType
		}
		if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
			return tftypes.Value{}, fmt.Errorf("'%v' expected a list, got unconvertible type '%T'", value, value)
		}
		elems := make([]tftypes.Value, 0, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			e, err := dkToTF(rv.Index(i).Interface(), et)
			if err != nil {
				return tftypes.Value{}, err
			}
			if t.Is(tftypes.Set{}) && dkContainsValue(elems, e) {
				continue
			}
			elems = append(elems, e)
		}
		return tftypes.NewValue(t, elems), nil
	case t.Is(tftypes.Map{}):
		et := t.(tftypes.Map).ElementType
		if rv.Kind() != reflect.Map {
			return tftypes.Value{}, fmt.Errorf("'%v' expected a map, got unconvertible type '%T'", value, value)
		}
		elems := make(map[string]tftypes.Value, rv.Len())
		iter := rv.MapRange()
		for iter.Next() {
			e, err := dkToTF(iter.Value().Interface(), et)
			if err != nil {
				return tftypes.Value{}, err
			}
			elems[fmt.Sprint(iter.Key().Interface())] = e
		}
		return tftypes.NewValue(t, elems), nil
	case t.Is(tftypes.Object{}):
		ot := t.(tftypes.Object)
		m, ok := value.(map[string]interface{})
		if !ok {
			return tftypes.Value{}, fmt.Errorf("'%v' expected an object, got unconvertible type '%T'", value, value)
		}
		for k := range m {
			if _, ok := ot.AttributeTypes[k]; !ok {
				return tftypes.Value{}, fmt.Errorf("invalid key: %s", k)
			}
		}
		attrs := make(map[string]tftypes.Value, len(ot.AttributeTypes))
		for k, at := range ot.AttributeTypes {
			e, err := dkToTF(m[k], at)
			if err != nil {
				return tftypes.Value{}, err
			}
			attrs[k] = e
		}
		return tftypes.NewValue(t, attrs), nil
	}
	return tftypes.Value{}, fmt.Errorf("unsupported type %s", t)
}

func dkContainsValue(list []tftypes.Value, v tftypes.Value) bool {
	for _, e := range list {
		if e.Equal(v) {
			return true
		}
	}
	return false
}

// dkIsZeroTF reports whether a value is null or the zero value of its type.
func dkIsZeroTF(v tftypes.Value) bool {
	if v.IsNull() {
		return true
	}
	if !v.IsFullyKnown() {
		return false
	}
	if v.Type().Is(tftypes.Object{}) {
		return false
	}
	return dkIsZeroGo(dkFromTF(v))
}

// dkSemanticallyEqual compares two values the way SDKv2 did: null equals
// the zero value of its type, recursively.
func dkSemanticallyEqual(a, b tftypes.Value) bool {
	if dkIsZeroTF(a) && dkIsZeroTF(b) {
		return true
	}
	if a.IsNull() || b.IsNull() || !a.IsKnown() || !b.IsKnown() {
		return a.Equal(b)
	}
	t := a.Type()
	switch {
	case t.Is(tftypes.List{}), t.Is(tftypes.Tuple{}):
		var ae, be []tftypes.Value
		_ = a.As(&ae)
		_ = b.As(&be)
		if len(ae) != len(be) {
			return false
		}
		for i := range ae {
			if !dkSemanticallyEqual(ae[i], be[i]) {
				return false
			}
		}
		return true
	case t.Is(tftypes.Map{}), t.Is(tftypes.Object{}):
		var am, bm map[string]tftypes.Value
		_ = a.As(&am)
		_ = b.As(&bm)
		for k, av := range am {
			bv, ok := bm[k]
			if !ok {
				if !dkIsZeroTF(av) {
					return false
				}
				continue
			}
			if !dkSemanticallyEqual(av, bv) {
				return false
			}
		}
		for k, bv := range bm {
			if _, ok := am[k]; !ok && !dkIsZeroTF(bv) {
				return false
			}
		}
		return true
	}
	return a.Equal(b)
}

// ---------------------------------------------------------------------------
// Building states
// ---------------------------------------------------------------------------

// dkAppliedState is the state after Create or Update: the planned value of
// every attribute the plan knows, and for the rest what the CRUD function
// set (null when it set nothing).
func dkAppliedState(b *dkBag, objType tftypes.Type, plan tftypes.Value) (tftypes.Value, error) {
	planAttrs := dkObjectAttrs(plan)
	out := map[string]tftypes.Value{}
	for name, t := range b.types {
		f := b.fields[name]
		pv, ok := planAttrs[name]
		switch {
		case name == "id":
			out[name] = dkStringValue(b.id)
		case f != nil && f.writeOnly:
			out[name] = tftypes.NewValue(t, nil)
		case ok && pv.IsFullyKnown():
			out[name] = pv
		case b.set[name]:
			v, err := dkToTF(b.vals[name], t)
			if err != nil {
				return tftypes.Value{}, fmt.Errorf("%s: %w", name, err)
			}
			out[name] = v
		default:
			out[name] = tftypes.NewValue(t, nil)
		}
	}
	return tftypes.NewValue(objType, out), nil
}

// dkReadState is the state after Read: what Read set, the prior state for
// the rest, with zero values of optional attributes written as null where
// SDKv2 would have treated them as unset.
func dkReadState(b *dkBag, objType tftypes.Type, prior tftypes.Value, frameworkWritten bool) (tftypes.Value, error) {
	priorAttrs := dkObjectAttrs(prior)
	out := map[string]tftypes.Value{}
	for name, t := range b.types {
		pv, ok := priorAttrs[name]
		if !ok {
			pv = tftypes.NewValue(t, nil)
		}
		if name == "id" {
			out[name] = dkStringValue(b.id)
			continue
		}
		nv := pv
		if b.set[name] {
			v, err := dkToTF(b.vals[name], t)
			if err != nil {
				return tftypes.Value{}, fmt.Errorf("%s: %w", name, err)
			}
			nv = v
		}
		out[name] = dkNormalizeRead(b.fields[name], nv, pv, frameworkWritten)
	}
	return tftypes.NewValue(objType, out), nil
}

func dkNormalizeRead(f *dkField, nv, pv tftypes.Value, frameworkWritten bool) tftypes.Value {
	t := nv.Type()
	if f == nil {
		return nv
	}
	if f.writeOnly {
		return tftypes.NewValue(t, nil)
	}
	if f.block {
		if nv.IsNull() || !nv.IsKnown() {
			return tftypes.NewValue(t, []tftypes.Value{})
		}
		var elems, priorElems []tftypes.Value
		_ = nv.As(&elems)
		if !pv.IsNull() && pv.IsKnown() {
			_ = pv.As(&priorElems)
		}
		var et tftypes.Type
		if lt, ok := t.(tftypes.List); ok {
			et = lt.ElementType
		} else if st, ok := t.(tftypes.Set); ok {
			et = st.ElementType
		}
		ot, _ := et.(tftypes.Object)
		outElems := make([]tftypes.Value, 0, len(elems))
		for i, e := range elems {
			attrs := dkObjectAttrs(e)
			var priorAttrs map[string]tftypes.Value
			if i < len(priorElems) {
				priorAttrs = dkObjectAttrs(priorElems[i])
			}
			nattrs := map[string]tftypes.Value{}
			for k, at := range ot.AttributeTypes {
				av, ok := attrs[k]
				if !ok {
					av = tftypes.NewValue(at, nil)
				}
				pav, ok := priorAttrs[k]
				if !ok {
					pav = tftypes.NewValue(at, nil)
				}
				nattrs[k] = dkNormalizeRead(f.nested[k], av, pav, frameworkWritten)
			}
			outElems = append(outElems, tftypes.NewValue(et, nattrs))
		}
		return tftypes.NewValue(t, outElems)
	}
	if f.optional && !f.computed && dkIsZeroTF(nv) && (pv.IsNull() || !frameworkWritten) {
		return tftypes.NewValue(t, nil)
	}
	return nv
}

// dkImportState is the state an importer leaves for Read: the attributes
// it set, null for the rest.
func dkImportState(b *dkBag, objType tftypes.Type) (tftypes.Value, error) {
	out := map[string]tftypes.Value{}
	for name, t := range b.types {
		switch {
		case name == "id":
			out[name] = dkStringValue(b.id)
		case b.set[name]:
			v, err := dkToTF(b.vals[name], t)
			if err != nil {
				return tftypes.Value{}, fmt.Errorf("%s: %w", name, err)
			}
			out[name] = v
		case b.fields[name] != nil && b.fields[name].block:
			out[name] = tftypes.NewValue(t, []tftypes.Value{})
		default:
			out[name] = tftypes.NewValue(t, nil)
		}
	}
	return tftypes.NewValue(objType, out), nil
}

func dkStringValue(s string) tftypes.Value {
	if s == "" {
		return tftypes.NewValue(tftypes.String, nil)
	}
	return tftypes.NewValue(tftypes.String, s)
}

// ---------------------------------------------------------------------------
// Plan modifiers
// ---------------------------------------------------------------------------

const dkReplaceDescription = "Changing this value replaces the resource; a change between an unset and an empty value does not."

func dkRequiresReplaceTF(ctx context.Context, state, plan attr.Value) bool {
	sv, err1 := state.ToTerraformValue(ctx)
	pv, err2 := plan.ToTerraformValue(ctx)
	if err1 != nil || err2 != nil {
		return true
	}
	return !dkSemanticallyEqual(sv, pv)
}

// dkReplaceString is ForceNew: the resource is replaced when the value
// changes, but not between "" and null, which SDKv2 treated as equal.
func dkReplaceString() planmodifier.String {
	return stringplanmodifier.RequiresReplaceIf(func(ctx context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
		resp.RequiresReplace = dkRequiresReplaceTF(ctx, req.StateValue, req.PlanValue)
	}, dkReplaceDescription, dkReplaceDescription)
}

func dkReplaceInt64() planmodifier.Int64 {
	return int64planmodifier.RequiresReplaceIf(func(ctx context.Context, req planmodifier.Int64Request, resp *int64planmodifier.RequiresReplaceIfFuncResponse) {
		resp.RequiresReplace = dkRequiresReplaceTF(ctx, req.StateValue, req.PlanValue)
	}, dkReplaceDescription, dkReplaceDescription)
}

func dkReplaceBool() planmodifier.Bool {
	return boolplanmodifier.RequiresReplaceIf(func(ctx context.Context, req planmodifier.BoolRequest, resp *boolplanmodifier.RequiresReplaceIfFuncResponse) {
		resp.RequiresReplace = dkRequiresReplaceTF(ctx, req.StateValue, req.PlanValue)
	}, dkReplaceDescription, dkReplaceDescription)
}

func dkReplaceMap() planmodifier.Map {
	return mapplanmodifier.RequiresReplaceIf(func(ctx context.Context, req planmodifier.MapRequest, resp *mapplanmodifier.RequiresReplaceIfFuncResponse) {
		resp.RequiresReplace = dkRequiresReplaceTF(ctx, req.StateValue, req.PlanValue)
	}, dkReplaceDescription, dkReplaceDescription)
}

func dkReplaceList() planmodifier.List {
	return listplanmodifier.RequiresReplaceIf(func(ctx context.Context, req planmodifier.ListRequest, resp *listplanmodifier.RequiresReplaceIfFuncResponse) {
		resp.RequiresReplace = dkRequiresReplaceTF(ctx, req.StateValue, req.PlanValue)
	}, dkReplaceDescription, dkReplaceDescription)
}

// ---------------------------------------------------------------------------
// Generic resource
// ---------------------------------------------------------------------------

// dkCRUDFunc is a CRUD function of a bag-backed resource.
type dkCRUDFunc func(ctx context.Context, d *dkBag, client *APIClient) error

// dkResourceDef describes a bag-backed resource.
type dkResourceDef struct {
	typeSuffix string // "_stack"
	schema     func(ctx context.Context) schema.Schema
	create     dkCRUDFunc
	read       dkCRUDFunc
	update     dkCRUDFunc // nil: every attribute forces a new resource
	delete     dkCRUDFunc // nil: removing it from state is all there is to do
	// importer adjusts the bag after the import ID was stored as its ID; nil
	// means the resource cannot be imported.
	importer dkCRUDFunc
	// modifyPlan is SDKv2's CustomizeDiff and the DiffSuppressFuncs.
	modifyPlan func(ctx context.Context, p *dkPlan) error
	// timeouts are the SDKv2 default timeouts per operation.
	timeouts         map[string]time.Duration
	configValidators func() []resource.ConfigValidator
	// identity is the resource identity (Terraform 1.12+); nil means none.
	// It needs an importer.
	identity *dkIdentityDef
}

type dkResource struct {
	def    *dkResourceDef
	client *APIClient
}

type dkImportableResource struct{ dkResource }

// dkIdentityResource is an importable bag-backed resource with an identity.
type dkIdentityResource struct{ dkImportableResource }

var (
	_ resource.ResourceWithConfigure        = (*dkResource)(nil)
	_ resource.ResourceWithModifyPlan       = (*dkResource)(nil)
	_ resource.ResourceWithConfigValidators = (*dkResource)(nil)
	_ resource.ResourceWithImportState      = (*dkImportableResource)(nil)
	_ resource.ResourceWithIdentity         = (*dkIdentityResource)(nil)
)

// dkNewResource returns the constructor of a bag-backed resource.
func dkNewResource(def *dkResourceDef) func() resource.Resource {
	return func() resource.Resource {
		if def.importer != nil && def.identity != nil {
			return &dkIdentityResource{dkImportableResource{dkResource{def: def}}}
		}
		if def.importer != nil {
			return &dkImportableResource{dkResource{def: def}}
		}
		return &dkResource{def: def}
	}
}

func (r *dkResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + r.def.typeSuffix
}

func (r *dkResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = r.def.schema(ctx)
}

func (r *dkResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *dkResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	if r.def.configValidators == nil {
		return nil
	}
	return r.def.configValidators()
}

func (r *dkResource) bag(ctx context.Context) (*dkBag, tftypes.Type) {
	s := r.def.schema(ctx)
	objType := s.Type().TerraformType(ctx)
	return newDkBag(dkResourceFields(s.Attributes, s.Blocks), objType), objType
}

// timeout is the context timeout SDKv2 gave an operation: the configured or
// declared one, or its 20 minute default.
func (r *dkResource) timeout(d *dkBag, op string) time.Duration {
	if def, declared := r.def.timeouts[op]; declared {
		return d.Timeout(op, def)
	}
	return dkDefaultTimeout
}

func dkAddError(diags *diag.Diagnostics, err error) {
	diags.AddError(err.Error(), "")
}

func (r *dkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	d, objType := r.bag(ctx)
	d.loadValues(req.Plan.Raw, false)
	d.loadConfig(req.Config.Raw)

	ctx, cancel := context.WithTimeout(ctx, r.timeout(d, "create"))
	defer cancel()
	err := r.def.create(ctx, d, r.client)
	if err != nil {
		dkAddError(&resp.Diagnostics, err)
		// SDKv2 kept a resource whose ID was set before the failure, tainted.
		if d.id == "" {
			return
		}
	} else if d.id == "" {
		resp.Diagnostics.AddError("Provider produced no resource ID", "The create operation finished without setting the resource ID.")
		return
	}
	state, convErr := dkAppliedState(d, objType, req.Plan.Raw)
	if convErr != nil {
		resp.Diagnostics.AddError("Failed to build the resource state", convErr.Error())
		return
	}
	resp.State.Raw = state
	r.setIdentity(ctx, d, resp.Identity, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.Private.SetKey(ctx, dkPrivateKey, []byte(`true`))...)
}

func (r *dkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	d, objType := r.bag(ctx)
	d.loadValues(req.State.Raw, true)
	// State written by v3.0.0 has no identity, and the framework rejects a
	// Read that leaves it null, even one that removes the resource: it comes
	// from the prior state first. A stored identity is never changed.
	if dkIdentityNull(resp.Identity) {
		r.setIdentity(ctx, d, resp.Identity, &resp.Diagnostics)
	}

	frameworkWritten := false
	if v, diags := req.Private.GetKey(ctx, dkPrivateKey); !diags.HasError() && len(v) > 0 {
		frameworkWritten = true
	}

	ctx, cancel := context.WithTimeout(ctx, r.timeout(d, "read"))
	defer cancel()
	if r.def.read != nil {
		if err := r.def.read(ctx, d, r.client); err != nil {
			dkAddError(&resp.Diagnostics, err)
			return
		}
	}
	if d.id == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	state, err := dkReadState(d, objType, req.State.Raw, frameworkWritten)
	if err != nil {
		resp.Diagnostics.AddError("Failed to build the resource state", err.Error())
		return
	}
	resp.State.Raw = state
	if dkIdentityNull(resp.Identity) {
		r.setIdentity(ctx, d, resp.Identity, &resp.Diagnostics)
	}
	// A list resource reads without private state (as after an import).
	if resp.Private != nil {
		resp.Diagnostics.Append(resp.Private.SetKey(ctx, dkPrivateKey, []byte(`true`))...)
	}
}

func (r *dkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.def.update == nil {
		resp.Diagnostics.AddError("Resource cannot be updated", "Every attribute of this resource forces a new resource.")
		return
	}
	d, objType := r.bag(ctx)
	d.loadValues(req.Plan.Raw, false)
	d.loadOld(req.State.Raw)
	d.loadConfig(req.Config.Raw)

	// A plan that only moves values between null and empty - a state
	// written by SDKv2 meeting a configuration that sets an empty value -
	// changes nothing in Portainer.
	if dkOnlyNullZeroChanges(req.Plan.Raw, req.State.Raw) {
		state, err := dkAppliedStateFromPrior(d, objType, req.Plan.Raw, req.State.Raw)
		if err != nil {
			resp.Diagnostics.AddError("Failed to build the resource state", err.Error())
			return
		}
		resp.State.Raw = state
		if dkIdentityNull(resp.Identity) {
			r.setIdentity(ctx, d, resp.Identity, &resp.Diagnostics)
		}
		resp.Diagnostics.Append(resp.Private.SetKey(ctx, dkPrivateKey, []byte(`true`))...)
		return
	}

	ctx, cancel := context.WithTimeout(ctx, r.timeout(d, "update"))
	defer cancel()
	err := r.def.update(ctx, d, r.client)
	if err != nil {
		dkAddError(&resp.Diagnostics, err)
	}
	state, convErr := dkAppliedState(d, objType, req.Plan.Raw)
	if convErr != nil {
		resp.Diagnostics.AddError("Failed to build the resource state", convErr.Error())
		return
	}
	resp.State.Raw = state
	if dkIdentityNull(resp.Identity) {
		r.setIdentity(ctx, d, resp.Identity, &resp.Diagnostics)
	}
	resp.Diagnostics.Append(resp.Private.SetKey(ctx, dkPrivateKey, []byte(`true`))...)
}

// dkOnlyNullZeroChanges reports whether a plan differs from the prior state
// only between null and zero values (computed values it leaves unknown aside).
func dkOnlyNullZeroChanges(plan, prior tftypes.Value) bool {
	priorAttrs := dkObjectAttrs(prior)
	for name, pv := range dkObjectAttrs(plan) {
		if name == "id" || name == "timeouts" || !pv.IsFullyKnown() {
			continue
		}
		sv, ok := priorAttrs[name]
		if !ok || !dkSemanticallyEqual(pv, sv) {
			return false
		}
	}
	return true
}

// dkAppliedStateFromPrior is the applied state of an update that sent
// nothing: the plan, with unknown computed values taken from the prior state.
func dkAppliedStateFromPrior(b *dkBag, objType tftypes.Type, plan, prior tftypes.Value) (tftypes.Value, error) {
	for name, v := range dkObjectAttrs(prior) {
		if name == "id" || b.set[name] {
			continue
		}
		b.vals[name] = dkFromTF(v)
		b.set[name] = !v.IsNull()
	}
	return dkAppliedState(b, objType, plan)
}

func (r *dkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.def.delete == nil {
		return
	}
	d, _ := r.bag(ctx)
	d.loadValues(req.State.Raw, true)
	ctx, cancel := context.WithTimeout(ctx, r.timeout(d, "delete"))
	defer cancel()
	if err := r.def.delete(ctx, d, r.client); err != nil {
		dkAddError(&resp.Diagnostics, err)
	}
}

func (r *dkImportableResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	r.importID(ctx, req.ID, resp)
}

// importID runs the importer on an import ID and stores the state it leaves
// (and the identity, for a resource with one).
func (r *dkImportableResource) importID(ctx context.Context, id string, resp *resource.ImportStateResponse) {
	d, objType := r.bag(ctx)
	d.id = id
	if err := r.def.importer(ctx, d, r.client); err != nil {
		dkAddError(&resp.Diagnostics, err)
		return
	}
	state, err := dkImportState(d, objType)
	if err != nil {
		resp.Diagnostics.AddError("Failed to build the imported state", err.Error())
		return
	}
	resp.State.Raw = state
	r.setIdentity(ctx, d, resp.Identity, &resp.Diagnostics)
}

func (r *dkIdentityResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = r.def.identity.schema()
}

// ImportState imports by the import ID or, from an import block with
// `identity`, by the import ID the identity stands for.
func (r *dkIdentityResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := req.ID
	if id == "" && req.Identity != nil && !req.Identity.Raw.IsNull() {
		values := r.def.identity.read(ctx, req.Identity, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
		var err error
		if id, err = r.def.identity.importID(ctx, r.client, values); err != nil {
			dkAddError(&resp.Diagnostics, err)
			return
		}
	}
	r.importID(ctx, id, resp)
}

// ---------------------------------------------------------------------------
// Plan modification
// ---------------------------------------------------------------------------

// dkPlan is the view of a plan CustomizeDiff and DiffSuppressFuncs had.
type dkPlan struct {
	*dkBag
	ctx     context.Context
	resp    *resource.ModifyPlanResponse
	creates bool
}

// ForceNew marks a top-level attribute as requiring replacement.
func (p *dkPlan) ForceNew(key string) {
	p.resp.RequiresReplace = append(p.resp.RequiresReplace, path.Root(key))
}

// SetPlanned changes the planned value of a top-level attribute.
func (p *dkPlan) SetPlanned(key string, value interface{}) error {
	t, ok := p.types[key]
	if !ok {
		return fmt.Errorf("unknown attribute %s", key)
	}
	v, err := dkToTF(value, t)
	if err != nil {
		return err
	}
	attrs := dkObjectAttrs(p.resp.Plan.Raw)
	if attrs == nil {
		return errors.New("no plan to modify")
	}
	attrs[key] = v
	p.resp.Plan.Raw = tftypes.NewValue(p.resp.Plan.Raw.Type(), attrs)
	p.vals[key] = dkFromTF(v)
	return nil
}

// PlannedValue returns the planned raw value of a top-level attribute.
func (p *dkPlan) PlannedValue(key string) tftypes.Value {
	if v, ok := dkObjectAttrs(p.resp.Plan.Raw)[key]; ok {
		return v
	}
	return tftypes.NewValue(p.types[key], nil)
}

// PriorValue returns the prior state's raw value of a top-level attribute.
func (p *dkPlan) PriorValue(key string) tftypes.Value {
	if v, ok := p.raw["__prior_"+key]; ok {
		return v
	}
	return tftypes.NewValue(p.types[key], nil)
}

func (r *dkResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if r.def.modifyPlan == nil || req.Plan.Raw.IsNull() {
		return
	}
	d, _ := r.bag(ctx)
	d.loadValues(req.Plan.Raw, false)
	d.loadOld(req.State.Raw)
	d.loadConfig(req.Config.Raw)
	for name, v := range dkObjectAttrs(req.State.Raw) {
		d.raw["__prior_"+name] = v
	}
	if req.State.Raw.IsNull() {
		d.id = ""
	}
	p := &dkPlan{dkBag: d, ctx: ctx, resp: resp, creates: req.State.Raw.IsNull()}
	if err := r.def.modifyPlan(ctx, p); err != nil {
		dkAddError(&resp.Diagnostics, err)
	}
}

// ---------------------------------------------------------------------------
// Generic data source
// ---------------------------------------------------------------------------

type dkDataSourceDef struct {
	typeSuffix string
	schema     func(ctx context.Context) dsschema.Schema
	read       dkCRUDFunc
}

type dkDataSource struct {
	def    *dkDataSourceDef
	client *APIClient
}

var _ datasource.DataSourceWithConfigure = (*dkDataSource)(nil)

func dkNewDataSource(def *dkDataSourceDef) func() datasource.DataSource {
	return func() datasource.DataSource { return &dkDataSource{def: def} }
}

func (s *dkDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + s.def.typeSuffix
}

func (s *dkDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = s.def.schema(ctx)
}

func (s *dkDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	s.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (s *dkDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	sch := s.def.schema(ctx)
	objType := sch.Type().TerraformType(ctx)
	fields := dkDataSourceFields(sch.Attributes)
	d := newDkBag(fields, objType)
	d.loadValues(req.Config.Raw, false)
	d.loadConfig(req.Config.Raw)

	ctx, cancel := context.WithTimeout(ctx, dkDefaultTimeout)
	defer cancel()
	if err := s.def.read(ctx, d, s.client); err != nil {
		dkAddError(&resp.Diagnostics, err)
		return
	}

	out := map[string]tftypes.Value{}
	for name, t := range d.types {
		f := fields[name]
		cv := d.ConfigValue(name)
		switch {
		case name == "id":
			out[name] = dkStringValue(d.id)
		case f != nil && !f.computed:
			out[name] = cv
		case f != nil && f.optional && !cv.IsNull():
			out[name] = cv
		case d.set[name] || (f != nil && f.optional):
			v, err := dkToTF(d.Get(name), t)
			if err != nil {
				resp.Diagnostics.AddError("Failed to build the data source state", fmt.Sprintf("%s: %s", name, err))
				return
			}
			out[name] = v
		default:
			out[name] = tftypes.NewValue(t, nil)
		}
	}
	resp.State.Raw = tftypes.NewValue(objType, out)
}

// ---------------------------------------------------------------------------
// Small helpers shared by the batch
// ---------------------------------------------------------------------------

// dkInt reads an int attribute.
func dkInt(d *dkBag, key string) int {
	v, _ := d.Get(key).(int)
	return v
}

// dkString reads a string attribute.
func dkString(d *dkBag, key string) string {
	v, _ := d.Get(key).(string)
	return v
}

// dkBool reads a bool attribute.
func dkBool(d *dkBag, key string) bool {
	v, _ := d.Get(key).(bool)
	return v
}

// dkStateNull reports whether the plan or state the bag was built from holds
// null for key (the configuration leaves it out).
func dkStateNull(d *dkBag, key string) bool {
	v, ok := d.raw[key]
	return !ok || v.IsNull()
}

// dkList reads a list or set attribute.
func dkList(d *dkBag, key string) []interface{} {
	v, _ := d.Get(key).([]interface{})
	return v
}

// dkMap reads a map attribute.
func dkMap(d *dkBag, key string) map[string]interface{} {
	v, _ := d.Get(key).(map[string]interface{})
	return v
}

// dkStringSlice converts a list of strings.
func dkStringSlice(list []interface{}) []string {
	out := make([]string, len(list))
	for i, v := range list {
		out[i], _ = v.(string)
	}
	return out
}

// dkIntSlice converts a list of numbers.
func dkIntSlice(list []interface{}) []int {
	out := make([]int, len(list))
	for i, v := range list {
		out[i], _ = v.(int)
	}
	return out
}

// dkStringMap converts a map(string).
func dkStringMap(m map[string]interface{}) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = fmt.Sprintf("%v", v)
	}
	return out
}

// dkTwinSecret returns the secret in field, or the one in its write-only
// twin <field>_wo when <field>_wo_version is set (see secretValue).
func dkTwinSecret(d *dkBag, field string) (string, error) {
	if dkInt(d, field+"_wo_version") != 0 {
		if wo := d.ConfigString(field + "_wo"); wo != "" {
			if dkString(d, field) != "" {
				if err := d.Set(field, ""); err != nil {
					return "", err
				}
			}
			return wo, nil
		}
	}
	return dkString(d, field), nil
}

// dkListValueFromTF converts a tftypes list into a framework list of type t.
func dkListValueFromTF(ctx context.Context, t attr.Type, v tftypes.Value) (types.List, error) {
	av, err := t.ValueFromTerraform(ctx, v)
	if err != nil {
		return types.List{}, err
	}
	list, ok := av.(types.List)
	if !ok {
		return types.List{}, fmt.Errorf("expected a list, got %T", av)
	}
	return list, nil
}
