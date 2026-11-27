package internal

import (
	"context"
	"fmt"
	"math/big"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Helpers of the framework settings, backup, LDAP and miscellaneous
// resources (batch G).

// SDKv2 wrote "" / false / 0 into state for an optional attribute Read set
// from an empty API value, and treated that as equal to an unset (null)
// configuration value. The framework does not, so state written by v3.0.0
// would plan `"" -> null` for every such attribute. A state the framework has
// written carries gsbFrameworkStateKey in its private data; in one that does
// not, an empty API value is read back as null.
const gsbFrameworkStateKey = "g_framework_state"

type gsbPrivateReader interface {
	GetKey(ctx context.Context, key string) ([]byte, diag.Diagnostics)
}

type gsbPrivateWriter interface {
	SetKey(ctx context.Context, key string, value []byte) diag.Diagnostics
}

// gsbFrameworkState reports whether the framework wrote this state (as
// opposed to v3.0.0 or an import).
func gsbFrameworkState(ctx context.Context, p gsbPrivateReader) bool {
	if p == nil {
		return false
	}
	v, _ := p.GetKey(ctx, gsbFrameworkStateKey)
	return len(v) > 0
}

// gsbMarkFrameworkState records that the framework wrote the state.
func gsbMarkFrameworkState(ctx context.Context, p gsbPrivateWriter, diags *diag.Diagnostics) {
	if p == nil {
		return
	}
	diags.Append(p.SetKey(ctx, gsbFrameworkStateKey, []byte("true"))...)
}

// gsbString is the value of an optional (not computed) string read back from
// Portainer. An empty API value is null unless the framework stored an empty
// string from the configuration before (fw), so neither an unset attribute
// nor a v3.0.0 state plans `"" -> null`.
func gsbString(v string, prior types.String, fw bool) types.String {
	if v != "" {
		return types.StringValue(v)
	}
	if fw && !prior.IsNull() && !prior.IsUnknown() {
		return types.StringValue("")
	}
	return types.StringNull()
}

// gsbInt64 is gsbString for a number (0 is the empty value).
func gsbInt64(v int64, prior types.Int64, fw bool) types.Int64 {
	if v != 0 {
		return types.Int64Value(v)
	}
	if fw && !prior.IsNull() && !prior.IsUnknown() {
		return types.Int64Value(0)
	}
	return types.Int64Null()
}

// gsbRequiresReplaceString is RequiresReplace for an optional string whose
// empty and null values mean the same (SDKv2 did not tell them apart), so a
// state normalised by gsbString never replaces the resource.
func gsbRequiresReplaceString() planmodifier.String {
	return stringplanmodifier.RequiresReplaceIf(func(_ context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
		resp.RequiresReplace = req.PlanValue.IsUnknown() || req.PlanValue.ValueString() != req.StateValue.ValueString()
	}, "Changing this value forces resource recreation.", "Changing this value forces resource recreation.")
}

// gsbWriteOnlySecretAttribute is the framework form of writeOnlySecretSchema:
// the write-only twin of secretKey, which needs versionKey alongside it.
// Both names are siblings, so it works at the root and inside a block.
func gsbWriteOnlySecretAttribute(secretKey, versionKey, what string) schema.StringAttribute {
	return schema.StringAttribute{
		Optional:  true,
		Sensitive: true,
		WriteOnly: true,
		Validators: []validator.String{
			stringvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName(secretKey)),
			stringvalidator.AlsoRequires(path.MatchRelative().AtParent().AtName(versionKey)),
		},
		Description: fmt.Sprintf("Write-only %s (supports ephemeral values; never stored in Terraform state). Conflicts with `%s`. Requires Terraform 1.11 or later.", what, secretKey),
	}
}

// gsbWriteOnlyVersionAttribute is the framework form of writeOnlyVersionSchema.
func gsbWriteOnlyVersionAttribute(woKey string, forceNew bool) schema.Int64Attribute {
	desc := fmt.Sprintf("Version of `%s`. Increment it to send a new value.", woKey)
	a := schema.Int64Attribute{Optional: true, Description: desc}
	if forceNew {
		a.Description += " Changing it forces resource recreation."
		a.PlanModifiers = []planmodifier.Int64{int64planmodifier.RequiresReplace()}
	}
	return a
}

// gsbTwinSecret is secretValue for the framework: the write-only value at
// woPath when version is set and the configuration gives one, else plain.
// fromWriteOnly tells the caller to clear the plaintext from state, which is
// the point of moving to the write-only twin.
func gsbTwinSecret(ctx context.Context, config tfsdk.Config, plain types.String, woPath path.Path, version types.Int64, diags *diag.Diagnostics) (value string, fromWriteOnly bool) {
	if !version.IsNull() && !version.IsUnknown() && version.ValueInt64() != 0 {
		if wo := writeOnlyConfigString(ctx, config, woPath, diags); wo != "" {
			return wo, true
		}
	}
	return plain.ValueString(), false
}

// gsbFillUnknown replaces every unknown value of planned (the plan of an
// apply) with the value at the same path of read (what Portainer reports
// after the write), or with the zero value of its type when read has none.
// Known planned values are kept as they are: the framework rejects a value
// that changes during apply, and a difference shows on the next refresh.
func gsbFillUnknown(planned, read tftypes.Value) (tftypes.Value, error) {
	return tftypes.Transform(planned, func(p *tftypes.AttributePath, v tftypes.Value) (tftypes.Value, error) {
		if v.IsKnown() {
			return v, nil
		}
		if r, _, err := tftypes.WalkAttributePath(read, p); err == nil {
			if rv, ok := r.(tftypes.Value); ok && rv.IsFullyKnown() && rv.Type().Equal(v.Type()) {
				return rv, nil
			}
		}
		return gsbZeroValue(v.Type()), nil
	})
}

// gsbZeroValue is the value SDKv2 stored for an unset attribute of type t.
func gsbZeroValue(t tftypes.Type) tftypes.Value {
	switch {
	case t.Is(tftypes.String):
		return tftypes.NewValue(t, "")
	case t.Is(tftypes.Bool):
		return tftypes.NewValue(t, false)
	case t.Is(tftypes.Number):
		return tftypes.NewValue(t, new(big.Float))
	case t.Is(tftypes.List{}), t.Is(tftypes.Set{}):
		return tftypes.NewValue(t, []tftypes.Value{})
	case t.Is(tftypes.Map{}):
		return tftypes.NewValue(t, map[string]tftypes.Value{})
	}
	return tftypes.NewValue(t, nil)
}
