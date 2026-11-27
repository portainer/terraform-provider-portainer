package internal

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Helpers of the framework GitOps workflow, workflow reconcile and source
// resources.

// gitopsResAddError reports err the way SDKv2's diag.FromErr did: the error
// text as the summary.
func gitopsResAddError(diags *diag.Diagnostics, err error) {
	diags.AddError(err.Error(), "")
}

// gitopsResString is the value of a string, "" for null or unknown.
func gitopsResString(v types.String) string {
	if v.IsNull() || v.IsUnknown() {
		return ""
	}
	return v.ValueString()
}

// gitopsResStrings reads a list(string) (nil for null/unknown).
func gitopsResStrings(ctx context.Context, l types.List, diags *diag.Diagnostics) []string {
	if l.IsNull() || l.IsUnknown() {
		return nil
	}
	var out []string
	diags.Append(l.ElementsAs(ctx, &out, false)...)
	return out
}

// gitopsResListLen is the number of elements of a known list, 0 otherwise.
func gitopsResListLen(l types.List) int {
	if l.IsNull() || l.IsUnknown() {
		return 0
	}
	return len(l.Elements())
}

// gitopsResObjects decodes a list of nested blocks into target (a pointer to a
// slice of models). Null or unknown lists leave it empty.
func gitopsResObjects(ctx context.Context, l types.List, target interface{}, diags *diag.Diagnostics) {
	if l.IsNull() || l.IsUnknown() {
		return
	}
	diags.Append(l.ElementsAs(ctx, target, false)...)
}

func gitopsResErrorf(diags *diag.Diagnostics, format string, args ...interface{}) {
	gitopsResAddError(diags, fmt.Errorf(format, args...))
}

// gitopsNullZeroes returns v with the zero values SDKv2 stored for unset
// optional arguments turned into null: false, 0, "" and empty lists or maps
// of primitives. Lists of objects are nested blocks, which stay lists; the
// attributes of their objects named in keep are left alone.
func gitopsNullZeroes(v attr.Value, keep map[string]bool) attr.Value {
	if v.IsNull() || v.IsUnknown() {
		return v
	}
	switch value := v.(type) {
	case types.Bool:
		if !value.ValueBool() {
			return types.BoolNull()
		}
	case types.Int64:
		if value.ValueInt64() == 0 {
			return types.Int64Null()
		}
	case types.String:
		if value.ValueString() == "" {
			return types.StringNull()
		}
	case types.Map:
		if len(value.Elements()) == 0 {
			return types.MapNull(value.ElementType(context.Background()))
		}
	case types.List:
		elemType := value.ElementType(context.Background())
		if _, isObject := elemType.(types.ObjectType); !isObject {
			if len(value.Elements()) == 0 {
				return types.ListNull(elemType)
			}
			return value
		}
		elems := make([]attr.Value, 0, len(value.Elements()))
		for _, e := range value.Elements() {
			elems = append(elems, gitopsNullZeroes(e, keep))
		}
		return types.ListValueMust(elemType, elems)
	case types.Object:
		attrs := make(map[string]attr.Value, len(value.Attributes()))
		for name, a := range value.Attributes() {
			if keep[name] {
				attrs[name] = a
				continue
			}
			attrs[name] = gitopsNullZeroes(a, keep)
		}
		return types.ObjectValueMust(value.AttributeTypes(context.Background()), attrs)
	}
	return v
}
