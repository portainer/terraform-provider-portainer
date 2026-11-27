package internal

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Helpers of the framework edge, D2K and KubeSolo resources and data sources.

// edgeStringFromAPI is the value of an optional (not computed) string read
// back from Portainer: an empty API value stays null when the attribute was
// null before, so an unset attribute never plans "" -> null.
func edgeStringFromAPI(prior types.String, v string) types.String {
	if v == "" && prior.IsNull() {
		return types.StringNull()
	}
	return types.StringValue(v)
}

// edgeInt64FromAPI is edgeStringFromAPI for a number.
func edgeInt64FromAPI(prior types.Int64, v int64) types.Int64 {
	if v == 0 && prior.IsNull() {
		return types.Int64Null()
	}
	return types.Int64Value(v)
}

// edgeIntListFromAPI is edgeStringFromAPI for a list(number): an empty list
// from the API keeps a null attribute null, and an empty one empty.
func edgeIntListFromAPI(prior types.List, v []int) types.List {
	if len(v) == 0 && (prior.IsNull() || prior.IsUnknown()) {
		return types.ListNull(types.Int64Type)
	}
	return int64ListValue(v)
}

// edgeStringList builds a list(string); nil gives an empty list, as SDKv2
// stored a computed list.
func edgeStringList(values []string) types.List {
	elems := make([]attr.Value, 0, len(values))
	for _, v := range values {
		elems = append(elems, types.StringValue(v))
	}
	return types.ListValueMust(types.StringType, elems)
}

// edgeObjectList builds a computed list(object) of the given attribute
// types, the protocol type SDKv2 reported for a computed list of resources.
func edgeObjectList(attrTypes map[string]attr.Type, items []map[string]attr.Value, diags *diag.Diagnostics) types.List {
	objType := types.ObjectType{AttrTypes: attrTypes}
	elems := make([]attr.Value, 0, len(items))
	for _, item := range items {
		obj, d := types.ObjectValue(attrTypes, item)
		diags.Append(d...)
		elems = append(elems, obj)
	}
	list, d := types.ListValue(objType, elems)
	diags.Append(d...)
	return list
}

// edgeIntsToJSON renders ints as a JSON array for a multipart form field.
func edgeIntsToJSON(values []int) string {
	if values == nil {
		values = []int{}
	}
	data, _ := json.Marshal(values)
	return string(data)
}

// sha256File returns the lowercase hex-encoded SHA256 of the file contents
// at the given path. Used to detect changes to the underlying edge config
// file when the file_path itself hasn't changed (issue #116) — the Portainer
// API doesn't expose the uploaded file content or any digest, so we track
// our locally computed hash in state.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// edgeFileHashPlan is the framework form of the CustomizeDiff the edge
// configuration and KubeSolo bundle resources share: the file at file_path is
// hashed during plan into file_sha256, so a file rewritten in place plans an
// update although file_path did not change.
func edgeFileHashPlan(ctx context.Context, plan *tfsdk.Plan, diags *diag.Diagnostics) {
	if plan.Raw.IsNull() {
		return
	}
	var fp types.String
	diags.Append(plan.GetAttribute(ctx, path.Root("file_path"), &fp)...)
	if fp.IsUnknown() || fp.IsNull() || fp.ValueString() == "" {
		return
	}
	hash, err := sha256File(fp.ValueString())
	if err != nil {
		diags.AddError("Invalid file_path", fmt.Sprintf("failed to hash file_path %q: %s", fp.ValueString(), err))
		return
	}
	diags.Append(plan.SetAttribute(ctx, path.Root("file_sha256"), types.StringValue(hash))...)
}

// edgePostMultipart sends a multipart body built by the caller and returns
// the status code and response body. Errors are transport errors only.
func edgePostMultipart(ctx context.Context, client *APIClient, method, url, contentType string, body *bytes.Buffer, legacyAuth bool) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return 0, nil, err
	}
	if legacyAuth {
		// The edge configuration endpoints were written against the two
		// credentials directly rather than setAuthHeader; kept as they were.
		if client.APIKey != "" {
			req.Header.Set("X-API-Key", client.APIKey)
		} else if client.JWTToken != "" {
			req.Header.Set("Authorization", "Bearer "+client.JWTToken)
		}
	} else if err := setAuthHeader(req, client); err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, data, nil
}

// edgeIntListsEqual compares two list(number) the way SDKv2's HasChange did:
// null and empty are the same.
func edgeIntListsEqual(ctx context.Context, a, b types.List, diags *diag.Diagnostics) bool {
	x, y := int64ListToInts(ctx, a, diags), int64ListToInts(ctx, b, diags)
	if len(x) != len(y) {
		return false
	}
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

// edgeKeepCreatedOnError stores a resource whose create call succeeded but
// whose follow-up failed, so Terraform keeps it as tainted and replaces it on
// the next apply rather than losing track of it - what SDKv2 did with an ID
// set before an error. The plan is stored with its unknown values nulled.
func edgeKeepCreatedOnError(ctx context.Context, plan tfsdk.Plan, id string, state *tfsdk.State, diags *diag.Diagnostics) {
	if !diags.HasError() || !state.Raw.IsNull() {
		return
	}
	raw, err := tftypes.Transform(plan.Raw, func(_ *tftypes.AttributePath, v tftypes.Value) (tftypes.Value, error) {
		if !v.IsKnown() {
			return tftypes.NewValue(v.Type(), nil), nil
		}
		return v, nil
	})
	if err != nil {
		return
	}
	state.Raw = raw
	diags.Append(state.SetAttribute(ctx, path.Root("id"), types.StringValue(id))...)
}

// The edgeOptionalZero* plan modifiers serve an optional attribute that SDKv2
// filled from the API with a zero value ("", 0, false) although the
// configuration left it out, which a framework Optional attribute cannot hold
// without planning a change. The attribute is made Optional+Computed and,
// while the configuration leaves it out, planned as:
//   - unknown on create, as Portainer may fill it in;
//   - the prior value when that is null or the zero value (no change);
//   - unknown otherwise, so the update reads back what Portainer keeps.

type edgeOptionalZeroString struct{}

func (edgeOptionalZeroString) Description(context.Context) string {
	return "Keeps a zero or null prior value while the configuration leaves the attribute out."
}

func (m edgeOptionalZeroString) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (edgeOptionalZeroString) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if !req.ConfigValue.IsNull() {
		return
	}
	switch {
	case req.State.Raw.IsNull():
		resp.PlanValue = types.StringUnknown()
	case req.StateValue.IsNull() || req.StateValue.ValueString() == "":
		resp.PlanValue = req.StateValue
	default:
		resp.PlanValue = types.StringUnknown()
	}
}

type edgeOptionalZeroInt64 struct{}

func (edgeOptionalZeroInt64) Description(context.Context) string {
	return "Keeps a zero or null prior value while the configuration leaves the attribute out."
}

func (m edgeOptionalZeroInt64) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (edgeOptionalZeroInt64) PlanModifyInt64(_ context.Context, req planmodifier.Int64Request, resp *planmodifier.Int64Response) {
	if !req.ConfigValue.IsNull() {
		return
	}
	switch {
	case req.State.Raw.IsNull():
		resp.PlanValue = types.Int64Unknown()
	case req.StateValue.IsNull() || req.StateValue.ValueInt64() == 0:
		resp.PlanValue = req.StateValue
	default:
		resp.PlanValue = types.Int64Unknown()
	}
}

type edgeOptionalZeroBool struct{}

func (edgeOptionalZeroBool) Description(context.Context) string {
	return "Keeps a zero or null prior value while the configuration leaves the attribute out."
}

func (m edgeOptionalZeroBool) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (edgeOptionalZeroBool) PlanModifyBool(_ context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	if !req.ConfigValue.IsNull() {
		return
	}
	switch {
	case req.State.Raw.IsNull():
		resp.PlanValue = types.BoolUnknown()
	case req.StateValue.IsNull() || !req.StateValue.ValueBool():
		resp.PlanValue = req.StateValue
	default:
		resp.PlanValue = types.BoolUnknown()
	}
}

// edgeRequiresReplaceString is the SDKv2 ForceNew of an optional string: a
// change between null and "" is no change, and an attribute the
// configuration leaves out counts as "" (what SDKv2 compared), even while its
// planned value is unknown.
func edgeRequiresReplaceString() planmodifier.String {
	return stringplanmodifier.RequiresReplaceIf(func(_ context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
		planned := req.PlanValue
		if planned.IsUnknown() {
			if !req.ConfigValue.IsNull() {
				return
			}
			planned = types.StringValue("")
		}
		resp.RequiresReplace = planned.ValueString() != req.StateValue.ValueString()
	}, "Changing this value forces resource recreation.", "Changing this value forces resource recreation.")
}

// edgeDataSourceBase carries the client of a framework data source of this
// batch and configures it.
type edgeDataSourceBase struct{ client *APIClient }

func (d *edgeDataSourceBase) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

// edgeSortedStrings returns a sorted copy, so the output is stable between
// two plans.
func edgeSortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}
