package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Resource identity (Terraform 1.12+) of the bag-backed Docker and stack
// resources. Their import IDs are composite ("<endpoint_id>:<id>"), so the
// identity has one attribute per part, each required for import; an import
// by identity builds the import ID from it and runs the usual importer.

// dkIdentityAttr is one identity attribute: a number (int64) or a string.
type dkIdentityAttr struct {
	name        string
	number      bool
	description string
}

// dkIdentityDef is the identity of a bag-backed resource.
type dkIdentityDef struct {
	attrs []dkIdentityAttr
	// values reads the identity from a bag (prior state, or what create,
	// import or read left): int64 for numbers, string otherwise. ok is false
	// when a part is missing.
	values func(d *dkBag) (values map[string]any, ok bool)
	// importID builds the import ID the importer takes from an identity.
	importID func(ctx context.Context, client *APIClient, values map[string]any) (string, error)
}

func (def *dkIdentityDef) schema() identityschema.Schema {
	attrs := make(map[string]identityschema.Attribute, len(def.attrs))
	for _, a := range def.attrs {
		if a.number {
			attrs[a.name] = identityschema.Int64Attribute{RequiredForImport: true, Description: a.description}
		} else {
			attrs[a.name] = identityschema.StringAttribute{RequiredForImport: true, Description: a.description}
		}
	}
	return identityschema.Schema{Attributes: attrs}
}

// read returns the values of an identity (from an import block).
func (def *dkIdentityDef) read(ctx context.Context, identity *tfsdk.ResourceIdentity, diags *diag.Diagnostics) map[string]any {
	out := make(map[string]any, len(def.attrs))
	for _, a := range def.attrs {
		if a.number {
			var v types.Int64
			diags.Append(identity.GetAttribute(ctx, path.Root(a.name), &v)...)
			out[a.name] = v.ValueInt64()
		} else {
			var v types.String
			diags.Append(identity.GetAttribute(ctx, path.Root(a.name), &v)...)
			out[a.name] = v.ValueString()
		}
	}
	return out
}

// set stores values as the identity.
func (def *dkIdentityDef) set(ctx context.Context, values map[string]any, identity *tfsdk.ResourceIdentity, diags *diag.Diagnostics) {
	for _, a := range def.attrs {
		diags.Append(identity.SetAttribute(ctx, path.Root(a.name), values[a.name])...)
	}
}

// setIdentity stores the identity of the bag, when the resource has one and
// the bag holds every part of it.
func (r *dkResource) setIdentity(ctx context.Context, d *dkBag, identity *tfsdk.ResourceIdentity, diags *diag.Diagnostics) {
	if r.def.identity == nil || identity == nil {
		return
	}
	values, ok := r.def.identity.values(d)
	if !ok {
		return
	}
	r.def.identity.set(ctx, values, identity, diags)
}

// dkIdentityNull reports whether an identity is there to be filled in.
func dkIdentityNull(identity *tfsdk.ResourceIdentity) bool {
	return identity != nil && identity.Raw.IsFullyNull()
}

const dkEndpointIdentityDescription = "ID of the Portainer environment (endpoint), the resource's endpoint_id."

// dkEndpointIdentity is the identity { endpoint_id, <key> } of an object
// imported as "<endpoint_id>:<key>"; key reads the second part from a bag.
func dkEndpointIdentity(name, description string, key func(d *dkBag) string) *dkIdentityDef {
	return &dkIdentityDef{
		attrs: []dkIdentityAttr{
			{name: "endpoint_id", number: true, description: dkEndpointIdentityDescription},
			{name: name, description: description},
		},
		values: func(d *dkBag) (map[string]any, bool) {
			endpointID, k := dkInt(d, "endpoint_id"), key(d)
			if endpointID == 0 || k == "" {
				return nil, false
			}
			return map[string]any{"endpoint_id": int64(endpointID), name: k}, true
		},
		importID: func(_ context.Context, _ *APIClient, v map[string]any) (string, error) {
			return fmt.Sprintf("%d:%s", v["endpoint_id"], v[name]), nil
		},
	}
}

func dkBagID(d *dkBag) string { return d.ID() }

func dkBagString(key string) func(d *dkBag) string {
	return func(d *dkBag) string { return dkString(d, key) }
}

var (
	dockerNetworkIdentity = dkEndpointIdentity("id", "Docker network ID, the resource's id attribute.", dkBagID)
	dockerConfigIdentity  = dkEndpointIdentity("id", "Docker Swarm config ID, the resource's id attribute.", dkBagID)
	dockerSecretIdentity  = dkEndpointIdentity("id", "Docker Swarm secret ID, the resource's id attribute.", dkBagID)
	dockerVolumeIdentity  = dkEndpointIdentity("name", "Docker volume name, the resource's name attribute.", dkBagString("name"))
	dockerPluginIdentity  = dkEndpointIdentity("name", "Docker plugin name (alias) as Docker reports it without the :latest tag, the resource's id and name.", dkBagID)
	dockerNodeIdentity    = dkEndpointIdentity("node_id", "Docker Swarm node ID, the resource's node_id attribute.", dkBagString("node_id"))
)

// stackIdentity is { endpoint_id, id, deployment_type }. The import ID also
// needs the method, which Portainer does not record: an import by identity
// takes "repository" for a stack deployed from Git and "string" for any
// other (Read then fills stack_file_content from Portainer's stack file).
var stackIdentity = &dkIdentityDef{
	attrs: []dkIdentityAttr{
		{name: "endpoint_id", number: true, description: dkEndpointIdentityDescription},
		{name: "id", description: "Numeric Portainer stack ID, the resource's id attribute."},
		{name: "deployment_type", description: "Deployment mode of the stack: standalone, swarm or kubernetes."},
	},
	values: func(d *dkBag) (map[string]any, bool) {
		endpointID, typ := dkInt(d, "endpoint_id"), dkString(d, "deployment_type")
		if endpointID == 0 || d.ID() == "" || typ == "" {
			return nil, false
		}
		return map[string]any{"endpoint_id": int64(endpointID), "id": d.ID(), "deployment_type": typ}, true
	},
	importID: func(ctx context.Context, client *APIClient, v map[string]any) (string, error) {
		stackID, err := strconv.Atoi(fmt.Sprint(v["id"]))
		if err != nil {
			return "", fmt.Errorf("invalid stack identity: id %q is not a number", v["id"])
		}
		method, err := stackInferMethod(ctx, client, stackID)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d:%d:%s:%s", v["endpoint_id"], stackID, v["deployment_type"], method), nil
	},
}

// stackInferMethod is the method an import by identity (or a list result)
// gives a stack: "repository" when it was deployed from Git, else "string".
func stackInferMethod(ctx context.Context, client *APIClient, stackID int) (string, error) {
	if client == nil {
		return "", fmt.Errorf("the provider is not configured")
	}
	var stack struct {
		GitConfig *struct{} `json:"gitConfig"`
	}
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/stacks/%d", client.Endpoint, stackID), nil, &stack); err != nil {
		return "", fmt.Errorf("failed to read stack %d: %w", stackID, err)
	}
	if stack.GitConfig != nil {
		return "repository", nil
	}
	return "string", nil
}
