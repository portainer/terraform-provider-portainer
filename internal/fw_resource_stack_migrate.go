package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// portainer_stack_migrate moves a stack to another environment once, at
// create. Read and Delete do nothing.

var newStackMigrateResource = dkNewResource(&dkResourceDef{
	typeSuffix: "_stack_migrate",
	schema:     stackMigrateSchema,
	create:     stackMigrateCreate,
})

func stackMigrateSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":                 schema.StringAttribute{Computed: true, Description: "Identifier of the resource.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"is_helm":            schema.BoolAttribute{Optional: true, Description: "Whether the stack is a Helm release on Kubernetes. Business Edition only. Only sent when set to true.", PlanModifiers: []planmodifier.Bool{dkReplaceBool()}},
			"endpoint_id":        schema.Int64Attribute{Optional: true, Description: "Optional source environment (endpoint) identifier. Required for stacks created before Portainer 1.18.0.", PlanModifiers: []planmodifier.Int64{dkReplaceInt64()}},
			"stack_id":           schema.Int64Attribute{Required: true, Description: "Stack identifier to migrate.", PlanModifiers: []planmodifier.Int64{dkReplaceInt64()}},
			"stack_name":         schema.StringAttribute{Optional: true, Description: "New name for the stack after migration. If not set, the original name is kept.", PlanModifiers: []planmodifier.String{dkReplaceString()}},
			"swarm_id":           schema.StringAttribute{Optional: true, Description: "Swarm cluster identifier (required when migrating to a Swarm environment).", PlanModifiers: []planmodifier.String{dkReplaceString()}},
			"target_endpoint_id": schema.Int64Attribute{Required: true, Description: "Target environment (endpoint) identifier to migrate the stack to.", PlanModifiers: []planmodifier.Int64{dkReplaceInt64()}},
		},
	}
}

func stackMigrateCreate(ctx context.Context, d *dkBag, client *APIClient) error {
	stackID := dkInt(d, "stack_id")
	targetEndpointID := dkInt(d, "target_endpoint_id")

	payload := map[string]interface{}{
		"EndpointID": targetEndpointID,
	}

	if v, ok := d.GetOk("stack_name"); ok {
		payload["Name"] = v.(string)
	}

	if v, ok := d.GetOk("swarm_id"); ok {
		payload["SwarmID"] = v.(string)
	}

	// Business Edition only; Community Edition ignores the field.
	if dkBool(d, "is_helm") {
		payload["IsHelm"] = true
	}

	path := fmt.Sprintf("/stacks/%d/migrate", stackID)
	if v, ok := d.GetOk("endpoint_id"); ok {
		path = fmt.Sprintf("%s?endpointId=%d", path, v.(int))
	}

	if err := doJSON(ctx, client, http.MethodPost, fmt.Sprintf("%s%s", client.Endpoint, path), payload, nil); err != nil {
		return fmt.Errorf("failed to migrate stack: %w", err)
	}

	d.SetID(strconv.Itoa(stackID) + "-" + strconv.FormatInt(time.Now().Unix(), 10))
	return nil
}
