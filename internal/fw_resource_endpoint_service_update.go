package internal

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource              = (*endpointServiceUpdateResource)(nil)
	_ resource.ResourceWithConfigure = (*endpointServiceUpdateResource)(nil)
)

// endpointServiceUpdateResource force-updates a Swarm service once, on
// creation. Read and Delete do nothing.
func newEndpointServiceUpdateResource() resource.Resource { return &endpointServiceUpdateResource{} }

type endpointServiceUpdateResource struct{ client *APIClient }

type endpointServiceUpdateModel struct {
	ID          types.String `tfsdk:"id"`
	EndpointID  types.Int64  `tfsdk:"endpoint_id"`
	ServiceName types.String `tfsdk:"service_name"`
	PullImage   types.Bool   `tfsdk:"pull_image"`
}

func (r *endpointServiceUpdateResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_endpoint_service_update"
}

func (r *endpointServiceUpdateResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"endpoint_id": schema.Int64Attribute{
				Required:      true,
				Description:   "Identifier of the Portainer endpoint hosting the Swarm service to force-update.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"service_name": schema.StringAttribute{
				Required:      true,
				Description:   "Name of the Swarm service that should be force-updated.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"pull_image": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Default:       booldefault.StaticBool(false),
				Description:   "Whether Portainer should pull the latest image when force-updating the service.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
			},
		},
	}
}

func (r *endpointServiceUpdateResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *endpointServiceUpdateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan endpointServiceUpdateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	endpointServiceUpdateRun(ctx, r.client, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// endpointServiceUpdateRun force-updates the service; it is the operation of
// both the resource and the portainer_endpoint_service_update action.
func endpointServiceUpdateRun(ctx context.Context, client *APIClient, plan *endpointServiceUpdateModel, diags *diag.Diagnostics) {
	endpointID := int(plan.EndpointID.ValueInt64())

	serviceID, err := resolveServiceID(ctx, client, endpointID, plan.ServiceName.ValueString())
	if err != nil {
		diags.AddError("Failed to update service", err.Error())
		return
	}

	payload := map[string]interface{}{
		"pullImage": plan.PullImage.ValueBool(),
		"serviceID": serviceID,
	}
	url := fmt.Sprintf("%s/endpoints/%d/forceupdateservice", client.Endpoint, endpointID)
	warnings := struct {
		Warnings []string `json:"Warnings"`
	}{}
	if err := doJSON(ctx, client, http.MethodPut, url, payload, &warnings); err != nil {
		diags.AddError("Failed to update service", fmt.Errorf("failed to update service: %w", err).Error())
		return
	}
	if len(warnings.Warnings) > 0 {
		log.Printf("[WARN] Service update warnings: %v", warnings.Warnings)
	}

	plan.ID = types.StringValue(strconv.Itoa(endpointID) + "-" + serviceID)
}

func (r *endpointServiceUpdateResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {
}

// Update is never called: every attribute forces a new resource.
func (r *endpointServiceUpdateResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Service updates cannot be updated", "Every attribute of portainer_endpoint_service_update forces a new resource.")
}

func (r *endpointServiceUpdateResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

func resolveServiceID(ctx context.Context, client *APIClient, endpointID int, name string) (string, error) {
	url := fmt.Sprintf("%s/endpoints/%d/docker/services", client.Endpoint, endpointID)
	var services []struct {
		ID   string `json:"ID"`
		Spec struct {
			Name string `json:"Name"`
		} `json:"Spec"`
	}
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &services); err != nil {
		return "", fmt.Errorf("failed to fetch services: %w", err)
	}

	for _, service := range services {
		if service.Spec.Name == name {
			return service.ID, nil
		}
	}

	return "", fmt.Errorf("service with name '%s' not found", name)
}
