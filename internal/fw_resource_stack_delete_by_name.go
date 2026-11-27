package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// portainer_stack_delete_by_name removes the Kubernetes stacks of a name
// once, at create. Read and Delete do nothing.

var newStackDeleteByNameResource = dkNewResource(&dkResourceDef{
	typeSuffix: "_stack_delete_by_name",
	schema:     stackDeleteByNameSchema,
	create:     stackDeleteByNameCreate,
})

func stackDeleteByNameSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Computed: true, Description: "Identifier of the resource.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"endpoint_id": schema.Int64Attribute{Required: true, Description: "Identifier of the environment the stack was deployed to.", PlanModifiers: []planmodifier.Int64{dkReplaceInt64()}},
			"external":    schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Whether the stack was created outside Portainer. Set it to true to remove a stack Portainer only discovered, rather than one it deployed.", PlanModifiers: []planmodifier.Bool{dkReplaceBool()}},
			"name":        schema.StringAttribute{Required: true, Description: "Name of the Kubernetes stack to remove. Every stack with this name in the target environment is removed.", PlanModifiers: []planmodifier.String{dkReplaceString()}},
			"namespace":   schema.StringAttribute{Required: true, Description: "Kubernetes namespace the stack was deployed into. Portainer requires it, even though its API specification does not list it.", PlanModifiers: []planmodifier.String{dkReplaceString()}},
		},
	}
}

func stackDeleteByNameCreate(ctx context.Context, d *dkBag, client *APIClient) error {
	name := dkString(d, "name")
	endpointID := dkInt(d, "endpoint_id")

	q := url.Values{}
	q.Set("endpointId", fmt.Sprint(endpointID))
	q.Set("namespace", dkString(d, "namespace"))
	if dkBool(d, "external") {
		q.Set("external", "true")
	}

	target := fmt.Sprintf("%s/stacks/name/%s?%s", client.Endpoint, url.PathEscape(name), q.Encode())
	if err := doJSON(ctx, client, http.MethodDelete, target, nil, nil); err != nil {
		// A stack that is already gone is the state this resource asks for.
		if !isAPINotFound(err) {
			return fmt.Errorf("failed to remove Kubernetes stack %q from environment %d: %w", name, endpointID, err)
		}
	}

	d.SetID(fmt.Sprintf("%d/%s/deleted/%d", endpointID, name, makeTimestamp()))
	return nil
}
