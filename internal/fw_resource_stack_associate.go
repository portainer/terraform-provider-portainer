package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// portainer_stack_associate adopts an orphaned stack into an environment.
// Destroying it only forgets it.

var newStackAssociateResource = dkNewResource(&dkResourceDef{
	typeSuffix: "_stack_associate",
	schema:     stackAssociateSchema,
	create:     stackAssociateCreate,
	read:       stackAssociateRead,
})

func stackAssociateSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":               schema.StringAttribute{Computed: true, Description: "Identifier of the resource.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"endpoint_id":      schema.Int64Attribute{Required: true, Description: "Identifier of the Portainer endpoint that should adopt the orphaned stack.", PlanModifiers: []planmodifier.Int64{dkReplaceInt64()}},
			"orphaned_running": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Whether the orphaned stack is already running on the endpoint; controls Portainer's adoption behaviour.", PlanModifiers: []planmodifier.Bool{dkReplaceBool()}},
			"stack_id":         schema.Int64Attribute{Required: true, Description: "Identifier of the orphaned Portainer stack to associate with an endpoint.", PlanModifiers: []planmodifier.Int64{dkReplaceInt64()}},
			"swarm_id":         schema.StringAttribute{Required: true, Description: "Swarm cluster identifier (`Cluster.ID`) of the target endpoint when associating a Swarm stack.", PlanModifiers: []planmodifier.String{dkReplaceString()}},
		},
	}
}

func stackAssociateCreate(ctx context.Context, d *dkBag, client *APIClient) error {
	stackID := dkInt(d, "stack_id")
	endpointID := dkInt(d, "endpoint_id")
	swarmID := dkString(d, "swarm_id")
	orphanedRunning := dkBool(d, "orphaned_running")

	apiURL := fmt.Sprintf("%s/stacks/%d/associate", client.Endpoint, stackID)

	params := url.Values{}
	params.Add("endpointId", strconv.Itoa(endpointID))
	params.Add("swarmId", swarmID)
	params.Add("orphanedRunning", strconv.FormatBool(orphanedRunning))

	if err := doJSON(ctx, client, http.MethodPut, apiURL+"?"+params.Encode(), nil, nil); err != nil {
		return fmt.Errorf("failed to associate stack %d to endpoint %d: %w", stackID, endpointID, err)
	}

	d.SetID(fmt.Sprintf("%d", stackID))
	return nil
}

// stackAssociateRead checks the association still holds: a stack
// deleted since drops out of state, and one now attached to another endpoint
// shows the change, which re-associates it.
func stackAssociateRead(ctx context.Context, d *dkBag, client *APIClient) error {
	var stack struct {
		EndpointID int `json:"EndpointId"`
	}
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/stacks/%s", client.Endpoint, d.ID()), nil, &stack); err != nil {
		if isAPINotFound(err) {
			d.SetID("")
			return nil
		}
		return fmt.Errorf("failed to read stack %s: %w", d.ID(), err)
	}
	if err := d.Set("endpoint_id", stack.EndpointID); err != nil {
		return err
	}
	return nil
}
