package internal

import (
	"context"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Shared helpers of the plugin-framework half of the provider. Add a helper
// here only when more than one resource needs it, and check for an existing
// one first.

// configuredClient returns the *APIClient the provider's Configure handed a
// framework resource, data source, ephemeral resource, action or list
// resource. It is nil before the provider is configured (Terraform validates
// configurations before that), which the callers tolerate.
func configuredClient(providerData any, diags *diag.Diagnostics) *APIClient {
	if providerData == nil {
		return nil
	}
	client, ok := providerData.(*APIClient)
	if !ok {
		diags.AddError("Unexpected provider data", fmt.Sprintf("expected *APIClient, got %T", providerData))
		return nil
	}
	return client
}

// writeOnlyConfigString reads a write-only attribute, which only the
// configuration carries (never the plan or the state). Null when unset.
func writeOnlyConfigString(ctx context.Context, config tfsdk.Config, p path.Path, diags *diag.Diagnostics) string {
	var v types.String
	diags.Append(config.GetAttribute(ctx, p, &v)...)
	if v.IsNull() || v.IsUnknown() {
		return ""
	}
	return v.ValueString()
}

// int64ListValue builds a list(number) from ints; an empty input gives an
// empty (not null) list, as SDKv2 stored it.
func int64ListValue(values []int) types.List {
	elems := make([]attr.Value, 0, len(values))
	for _, v := range values {
		elems = append(elems, types.Int64Value(int64(v)))
	}
	return types.ListValueMust(types.Int64Type, elems)
}

// int64ListToInts reads a list(number) into ints (nil for null/unknown).
func int64ListToInts(ctx context.Context, l types.List, diags *diag.Diagnostics) []int {
	if l.IsNull() || l.IsUnknown() {
		return nil
	}
	var v []int64
	diags.Append(l.ElementsAs(ctx, &v, false)...)
	out := make([]int, 0, len(v))
	for _, n := range v {
		out = append(out, int(n))
	}
	return out
}

// stringMapValue builds a map(string); an empty input gives an empty map.
func stringMapValue(m map[string]string) types.Map {
	elems := make(map[string]attr.Value, len(m))
	for k, v := range m {
		elems[k] = types.StringValue(v)
	}
	return types.MapValueMust(types.StringType, elems)
}

// stringMapToGo reads a map(string) (nil for null/unknown).
func stringMapToGo(ctx context.Context, m types.Map, diags *diag.Diagnostics) map[string]string {
	if m.IsNull() || m.IsUnknown() {
		return nil
	}
	out := map[string]string{}
	diags.Append(m.ElementsAs(ctx, &out, false)...)
	return out
}

// durationValidator is validateDuration for the framework: an empty string or
// a non-negative Go duration such as "90s".
type durationValidator struct{}

func (durationValidator) Description(context.Context) string {
	return "value must be a non-negative duration such as \"90s\" or \"2m\""
}

func (v durationValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (durationValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	key := req.Path.String()
	if _, errs := validateDuration(req.ConfigValue.ValueString(), key); len(errs) > 0 {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid duration", errs[0].Error())
	}
}

// httpURLValidator is the framework form of SDKv2's
// validation.IsURLWithHTTPorHTTPS, with the same checks and messages.
type httpURLValidator struct{}

func (httpURLValidator) Description(context.Context) string {
	return "value must be a URL with an http or https scheme and a host"
}

func (v httpURLValidator) MarkdownDescription(ctx context.Context) string { return v.Description(ctx) }

func (httpURLValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	v := req.ConfigValue.ValueString()
	k := req.Path.String()
	if v == "" {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid URL", fmt.Sprintf("expected %q url to not be empty, got %v", k, v))
		return
	}
	u, err := url.Parse(v)
	if err != nil {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid URL", fmt.Sprintf("expected %q to be a valid url, got %v: %+v", k, v, err))
		return
	}
	if u.Host == "" {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid URL", fmt.Sprintf("expected %q to have a host, got %v", k, v))
		return
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid URL", fmt.Sprintf("expected %q to have a url with schema of: %q, got %v", k, "http,https", v))
	}
}
