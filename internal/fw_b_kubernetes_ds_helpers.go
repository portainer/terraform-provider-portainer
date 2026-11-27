package internal

import (
	"context"
	"fmt"
	"math/big"
	"net/url"
	"reflect"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// The read-only Kubernetes data sources (portainer_kubernetes_*), served by
// the plugin framework. They only read: arguments in, API calls, computed
// values out. Each one is a k8sDataSource - a schema plus a read function -
// and the read functions keep the shape they had under SDKv2 through
// k8sDSData, which offers the d.Get / d.GetOk / d.Set / d.SetId calls with
// SDKv2's semantics (zero values for unset arguments, GetOk false for a zero
// value, typed nil lists and maps stored as empty ones), so the API calls,
// the values written to state and the IDs stay exactly what v3.0.0 produced.

// k8sDSReadFunc reads one data source. A returned error is reported the way
// SDKv2's diag.FromErr reported it: the whole text as the summary.
type k8sDSReadFunc func(ctx context.Context, client *APIClient, d *k8sDSData) error

type k8sDataSource struct {
	typeName string
	schema   schema.Schema
	// defaults are the SDKv2 Default values of optional arguments. A
	// framework data source schema has no defaults, so those arguments are
	// optional+computed and Read stores the default when the configuration
	// leaves them null, as SDKv2 did.
	defaults map[string]interface{}
	read     k8sDSReadFunc
	client   *APIClient
}

var _ datasource.DataSourceWithConfigure = (*k8sDataSource)(nil)

// newK8sDataSource builds the data source portainer_<typeName>. The schema's
// computed "id" is added here.
func newK8sDataSource(typeName string, s schema.Schema, defaults map[string]interface{}, read k8sDSReadFunc) datasource.DataSource {
	if _, ok := s.Attributes["id"]; !ok {
		s.Attributes["id"] = schema.StringAttribute{Description: "Identifier of the resource.", Computed: true}
	}
	return &k8sDataSource{typeName: typeName, schema: s, defaults: defaults, read: read}
}

func (k *k8sDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + k.typeName
}

func (k *k8sDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = k.schema
}

func (k *k8sDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	k.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (k *k8sDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	d := &k8sDSData{ctx: ctx, schema: k.schema, defaults: k.defaults, config: req.Config, state: &resp.State, diags: &resp.Diagnostics}
	for name, value := range k.defaults {
		if raw, _ := d.configValue(name); raw.IsNull() {
			if err := d.Set(name, value); err != nil {
				resp.Diagnostics.AddError(err.Error(), "")
				return
			}
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}
	if err := k.read(ctx, k.client, d); err != nil {
		resp.Diagnostics.AddError(err.Error(), "")
	}
}

// k8sDSData is the SDKv2 ResourceData surface the read functions use.
type k8sDSData struct {
	ctx      context.Context
	schema   schema.Schema
	defaults map[string]interface{}
	config   tfsdk.Config
	state    *tfsdk.State
	diags    *diag.Diagnostics
}

func (d *k8sDSData) attrType(name string) (attr.Type, error) {
	a, ok := d.schema.Attributes[name]
	if !ok {
		return nil, fmt.Errorf("%q is not an attribute of this data source", name)
	}
	return a.GetType(), nil
}

func (d *k8sDSData) configValue(name string) (tftypes.Value, error) {
	v, _, err := tftypes.WalkAttributePath(d.config.Raw, tftypes.NewAttributePath().WithAttributeName(name))
	if err != nil {
		return tftypes.Value{}, err
	}
	value, ok := v.(tftypes.Value)
	if !ok {
		return tftypes.Value{}, fmt.Errorf("%q: unexpected configuration value %T", name, v)
	}
	return value, nil
}

// Get is d.Get: the configured argument, its SDKv2 default, or the zero value
// of its type (0, "", false, an empty list).
func (d *k8sDSData) Get(name string) interface{} {
	t, err := d.attrType(name)
	if err != nil {
		d.diags.AddError(err.Error(), "")
		return nil
	}
	raw, err := d.configValue(name)
	if err != nil {
		d.diags.AddError(err.Error(), "")
		return k8sDSZero(t)
	}
	if raw.IsNull() || !raw.IsKnown() {
		if def, ok := d.defaults[name]; ok {
			return def
		}
		return k8sDSZero(t)
	}
	v, err := k8sDSFromTerraform(t, raw)
	if err != nil {
		d.diags.AddError(err.Error(), "")
		return k8sDSZero(t)
	}
	return v
}

// GetOk is d.GetOk: false when the argument is unset or holds its zero value.
func (d *k8sDSData) GetOk(name string) (interface{}, bool) {
	v := d.Get(name)
	if v == nil {
		return v, false
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice, reflect.Map:
		return v, rv.Len() > 0
	default:
		return v, !rv.IsZero()
	}
}

// Set is d.Set for a computed attribute.
func (d *k8sDSData) Set(name string, value interface{}) error {
	t, err := d.attrType(name)
	if err != nil {
		return err
	}
	v, err := k8sDSToValue(t, value)
	if err != nil {
		return fmt.Errorf("failed to set %q: %w", name, err)
	}
	diags := d.state.SetAttribute(d.ctx, path.Root(name), v)
	d.diags.Append(diags...)
	if diags.HasError() {
		return fmt.Errorf("failed to set %q", name)
	}
	return nil
}

// SetFields is setFields: d.Set for every entry.
func (d *k8sDSData) SetFields(fields map[string]interface{}) error {
	for k, v := range fields {
		if err := d.Set(k, v); err != nil {
			return err
		}
	}
	return nil
}

// SetID is d.SetId.
func (d *k8sDSData) SetID(id string) {
	d.diags.Append(d.state.SetAttribute(d.ctx, path.Root("id"), types.StringValue(id))...)
}

func k8sDSZero(t attr.Type) interface{} {
	switch t.(type) {
	case basetypes.StringType:
		return ""
	case basetypes.Int64Type:
		return 0
	case basetypes.Float64Type:
		return 0.0
	case basetypes.BoolType:
		return false
	case basetypes.ListType:
		return []interface{}{}
	case basetypes.MapType:
		return map[string]interface{}{}
	}
	return nil
}

// k8sDSFromTerraform turns a configured argument into the Go value d.Get
// returned for it.
func k8sDSFromTerraform(t attr.Type, v tftypes.Value) (interface{}, error) {
	if v.IsNull() || !v.IsKnown() {
		return k8sDSZero(t), nil
	}
	switch tt := t.(type) {
	case basetypes.StringType:
		var s string
		err := v.As(&s)
		return s, err
	case basetypes.BoolType:
		var b bool
		err := v.As(&b)
		return b, err
	case basetypes.Int64Type:
		var f big.Float
		if err := v.As(&f); err != nil {
			return nil, err
		}
		n, _ := f.Int64()
		return int(n), nil
	case basetypes.Float64Type:
		var f big.Float
		if err := v.As(&f); err != nil {
			return nil, err
		}
		n, _ := f.Float64()
		return n, nil
	case basetypes.ListType:
		var elems []tftypes.Value
		if err := v.As(&elems); err != nil {
			return nil, err
		}
		out := make([]interface{}, 0, len(elems))
		for _, e := range elems {
			ev, err := k8sDSFromTerraform(tt.ElemType, e)
			if err != nil {
				return nil, err
			}
			out = append(out, ev)
		}
		return out, nil
	}
	return nil, fmt.Errorf("unsupported argument type %s", t)
}

// k8sDSToValue converts what a read function passes to Set into a value of
// type t, the way SDKv2's field writer stored it: a nil list is an empty
// list, a typed nil map an empty map, an untyped nil map or primitive null,
// and a key missing from an object map a null attribute.
func k8sDSToValue(t attr.Type, value interface{}) (attr.Value, error) {
	if value == nil {
		switch tt := t.(type) {
		case basetypes.ListType:
			return types.ListValueMust(tt.ElemType, []attr.Value{}), nil
		default:
			return k8sDSNull(t)
		}
	}
	rv := reflect.ValueOf(value)
	switch tt := t.(type) {
	case basetypes.StringType:
		if rv.Kind() != reflect.String {
			return nil, fmt.Errorf("expected a string, got %T", value)
		}
		return types.StringValue(rv.String()), nil
	case basetypes.BoolType:
		if rv.Kind() != reflect.Bool {
			return nil, fmt.Errorf("expected a bool, got %T", value)
		}
		return types.BoolValue(rv.Bool()), nil
	case basetypes.Int64Type:
		switch rv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return types.Int64Value(rv.Int()), nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return types.Int64Value(int64(rv.Uint())), nil //nolint:gosec // counts and sizes, never above MaxInt64
		}
		return nil, fmt.Errorf("expected an integer, got %T", value)
	case basetypes.Float64Type:
		switch rv.Kind() {
		case reflect.Float32, reflect.Float64:
			return types.Float64Value(rv.Float()), nil
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return types.Float64Value(float64(rv.Int())), nil
		}
		return nil, fmt.Errorf("expected a number, got %T", value)
	case basetypes.ListType:
		if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
			return nil, fmt.Errorf("expected a list, got %T", value)
		}
		elems := make([]attr.Value, 0, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			e, err := k8sDSToValue(tt.ElemType, rv.Index(i).Interface())
			if err != nil {
				return nil, fmt.Errorf("[%d]: %w", i, err)
			}
			elems = append(elems, e)
		}
		list, diags := types.ListValue(tt.ElemType, elems)
		if diags.HasError() {
			return nil, fmt.Errorf("%v", diags)
		}
		return list, nil
	case basetypes.MapType:
		if rv.Kind() != reflect.Map {
			return nil, fmt.Errorf("expected a map, got %T", value)
		}
		elems := make(map[string]attr.Value, rv.Len())
		iter := rv.MapRange()
		for iter.Next() {
			e, err := k8sDSToValue(tt.ElemType, iter.Value().Interface())
			if err != nil {
				return nil, fmt.Errorf("[%q]: %w", iter.Key().String(), err)
			}
			elems[iter.Key().String()] = e
		}
		m, diags := types.MapValue(tt.ElemType, elems)
		if diags.HasError() {
			return nil, fmt.Errorf("%v", diags)
		}
		return m, nil
	case basetypes.ObjectType:
		fields, ok := value.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("expected an object, got %T", value)
		}
		attrs := make(map[string]attr.Value, len(tt.AttrTypes))
		for name, at := range tt.AttrTypes {
			raw, present := fields[name]
			var (
				v   attr.Value
				err error
			)
			if present {
				v, err = k8sDSToValue(at, raw)
			} else {
				v, err = k8sDSNull(at)
			}
			if err != nil {
				return nil, fmt.Errorf(".%s: %w", name, err)
			}
			attrs[name] = v
		}
		obj, diags := types.ObjectValue(tt.AttrTypes, attrs)
		if diags.HasError() {
			return nil, fmt.Errorf("%v", diags)
		}
		return obj, nil
	}
	return nil, fmt.Errorf("unsupported attribute type %s", t)
}

func k8sDSNull(t attr.Type) (attr.Value, error) {
	return t.ValueFromTerraform(context.Background(), tftypes.NewValue(t.TerraformType(context.Background()), nil))
}

// kubernetesListQueryFW is kubernetesListQuery for k8sDSData: the label and
// field selector query string shared by the Kubernetes list endpoints.
func kubernetesListQueryFW(d *k8sDSData, extra map[string]string) string {
	q := url.Values{}
	if v, ok := d.GetOk("label_selector"); ok {
		q.Set("labelSelector", v.(string))
	}
	if v, ok := d.GetOk("field_selector"); ok {
		q.Set("fieldSelector", v.(string))
	}
	for k, v := range extra {
		if v != "" {
			q.Set(k, v)
		}
	}
	if len(q) == 0 {
		return ""
	}
	return "?" + q.Encode()
}

// Schema shorthands. Nested object fields of a computed list keep their
// SDKv2 description next to them (k8sDSField.desc): Terraform protocol 5
// has no place for it, as it had none under SDKv2 either.

// k8sDSField is one field of a computed list's object.
type k8sDSField struct {
	name string
	typ  attr.Type
	desc string
}

func k8sDSObject(fields ...k8sDSField) types.ObjectType {
	attrs := make(map[string]attr.Type, len(fields))
	for _, f := range fields {
		_ = f.desc
		attrs[f.name] = f.typ
	}
	return types.ObjectType{AttrTypes: attrs}
}

// k8sDSObjectList is a computed list of objects, an attribute as SDKv2
// served a computed TypeList of *schema.Resource.
func k8sDSObjectList(desc string, fields ...k8sDSField) schema.ListAttribute {
	return schema.ListAttribute{Computed: true, Description: desc, ElementType: k8sDSObject(fields...)}
}

var (
	k8sDSStringList = types.ListType{ElemType: types.StringType}
	k8sDSStringMap  = types.MapType{ElemType: types.StringType}
	k8sDSIntMap     = types.MapType{ElemType: types.Int64Type}
)

func k8sDSComputedString(desc string) schema.StringAttribute {
	return schema.StringAttribute{Computed: true, Description: desc}
}

func k8sDSComputedInt(desc string) schema.Int64Attribute {
	return schema.Int64Attribute{Computed: true, Description: desc}
}

func k8sDSComputedFloat(desc string) schema.Float64Attribute {
	return schema.Float64Attribute{Computed: true, Description: desc}
}

func k8sDSComputedBool(desc string) schema.BoolAttribute {
	return schema.BoolAttribute{Computed: true, Description: desc}
}

func k8sDSComputedStringList(desc string) schema.ListAttribute {
	return schema.ListAttribute{Computed: true, Description: desc, ElementType: types.StringType}
}

func k8sDSComputedStringMap(desc string) schema.MapAttribute {
	return schema.MapAttribute{Computed: true, Description: desc, ElementType: types.StringType}
}

func k8sDSRequiredInt(desc string) schema.Int64Attribute {
	return schema.Int64Attribute{Required: true, Description: desc}
}

func k8sDSRequiredString(desc string) schema.StringAttribute {
	return schema.StringAttribute{Required: true, Description: desc}
}

func k8sDSOptionalString(desc string) schema.StringAttribute {
	return schema.StringAttribute{Optional: true, Description: desc}
}

// k8sDSOptionalBoolDefault is an optional bool SDKv2 defaulted (to false in
// every data source here); see k8sDataSource.defaults.
func k8sDSOptionalBoolDefault(desc string) schema.BoolAttribute {
	return schema.BoolAttribute{Optional: true, Computed: true, Description: desc}
}
