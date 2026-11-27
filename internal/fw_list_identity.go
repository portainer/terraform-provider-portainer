package internal

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Resource identity (Terraform 1.12+) and list resources (Terraform 1.14+,
// `terraform query`) of the long-lived resources whose import ID is their
// Portainer ID. The identity is that ID alone, so an import block can name
// the object with `identity = { id = "3" }` and a list result can be turned
// into an import block as is.

// fwIDIdentity gives a resource the identity { id }. Embed it in the resource
// type; the resource stores the identity with fwSetIDIdentity.
type fwIDIdentity struct{}

func (fwIDIdentity) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			"id": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Portainer identifier of the object, the same value as the resource's id attribute and import ID.",
			},
		},
	}
}

// fwSetIDIdentity stores the id attribute of st as the identity. Read calls
// it with the prior state before anything else: state written by v3.0.0 has
// no identity, and the framework rejects a Read that leaves it null, even one
// that removes the resource.
func fwSetIDIdentity(ctx context.Context, st tfsdk.State, identity *tfsdk.ResourceIdentity, diags *diag.Diagnostics) {
	if identity == nil || st.Raw.IsNull() {
		return
	}
	var id types.String
	diags.Append(st.GetAttribute(ctx, path.Root("id"), &id)...)
	if id.IsNull() || id.IsUnknown() || id.ValueString() == "" {
		return
	}
	diags.Append(identity.SetAttribute(ctx, path.Root("id"), id)...)
}

// fwImportIDOrIdentity imports by the import ID (unchanged) or, from an
// import block with `identity`, by its id.
func fwImportIDOrIdentity(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughWithIdentity(ctx, path.Root("id"), path.Root("id"), req, resp)
}

// fwIDListItem is one object a list resource found: its ID, the name shown by
// `terraform query`, and values Read needs besides the ID (seed).
type fwIDListItem struct {
	id   string
	name string
	seed map[string]any
}

// fwIDListResource is the list resource of a resource with the { id }
// identity. items lists the objects (applying the filters of the list
// configuration); the full resource, when asked for, comes from the
// resource's own Read, as after an import.
type fwIDListResource struct {
	typeSuffix  string
	description string
	newResource func() resource.Resource
	filters     map[string]listschema.Attribute
	items       func(ctx context.Context, client *APIClient, config tfsdk.Config, diags *diag.Diagnostics) []fwIDListItem

	client *APIClient
}

var (
	_ list.ListResource              = (*fwIDListResource)(nil)
	_ list.ListResourceWithConfigure = (*fwIDListResource)(nil)
)

// fwNewIDList returns the constructor of a list resource.
func fwNewIDList(def fwIDListResource) func() list.ListResource {
	return func() list.ListResource {
		r := def
		return &r
	}
}

func (r *fwIDListResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + r.typeSuffix
}

func (r *fwIDListResource) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{Description: r.description, Attributes: r.filters}
}

func (r *fwIDListResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *fwIDListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	var diags diag.Diagnostics
	if r.client == nil {
		diags.AddError("Provider not configured", "The list resource needs a configured portainer provider.")
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}
	items := r.items(ctx, r.client, req.Config, &diags)
	if diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	var res resource.Resource
	if req.IncludeResource {
		res = r.newResource()
		if c, ok := res.(resource.ResourceWithConfigure); ok {
			var cresp resource.ConfigureResponse
			c.Configure(ctx, resource.ConfigureRequest{ProviderData: r.client}, &cresp)
			if cresp.Diagnostics.HasError() {
				stream.Results = list.ListResultsStreamDiagnostics(cresp.Diagnostics)
				return
			}
		}
	}

	stream.Results = func(push func(list.ListResult) bool) {
		for _, item := range items {
			result := req.NewListResult(ctx)
			result.DisplayName = item.name
			result.Diagnostics.Append(result.Identity.SetAttribute(ctx, path.Root("id"), item.id)...)
			if res != nil && !result.Diagnostics.HasError() && !fwListReadResource(ctx, res, item, &result) {
				continue // gone between the listing and the read
			}
			if !push(result) {
				return
			}
		}
	}
}

// fwListReadResource fills result.Resource with what the resource's Read
// stores after importing the item. It reports false when Read found the
// object gone.
func fwListReadResource(ctx context.Context, res resource.Resource, item fwIDListItem, result *list.ListResult) bool {
	st := tfsdk.State{Schema: result.Resource.Schema, Raw: result.Resource.Raw}
	result.Diagnostics.Append(st.SetAttribute(ctx, path.Root("id"), item.id)...)
	for k, v := range item.seed {
		result.Diagnostics.Append(st.SetAttribute(ctx, path.Root(k), v)...)
	}
	if result.Diagnostics.HasError() {
		return true
	}

	readReq := resource.ReadRequest{State: st, Identity: result.Identity}
	readResp := resource.ReadResponse{State: st, Identity: result.Identity}
	res.Read(ctx, readReq, &readResp)
	result.Diagnostics.Append(readResp.Diagnostics...)
	if result.Diagnostics.HasError() {
		return true
	}
	if readResp.State.Raw.IsNull() {
		return false
	}
	result.Resource.Raw = readResp.State.Raw
	return true
}

// fwListFilter reads an optional string filter of the list configuration
// ("" when unset).
func fwListFilter(ctx context.Context, config tfsdk.Config, name string, diags *diag.Diagnostics) string {
	var v types.String
	diags.Append(config.GetAttribute(ctx, path.Root(name), &v)...)
	return v.ValueString()
}

// fwListFilterInt64 reads an optional number filter (0 when unset).
func fwListFilterInt64(ctx context.Context, config tfsdk.Config, name string, diags *diag.Diagnostics) int64 {
	var v types.Int64
	diags.Append(config.GetAttribute(ctx, path.Root(name), &v)...)
	return v.ValueInt64()
}

// fwListNameFilter is the usual optional exact-name filter.
func fwListNameFilter(what string) map[string]listschema.Attribute {
	return map[string]listschema.Attribute{
		"name": listschema.StringAttribute{
			Optional:    true,
			Description: fmt.Sprintf("Only list the %s with exactly this name.", what),
		},
	}
}
