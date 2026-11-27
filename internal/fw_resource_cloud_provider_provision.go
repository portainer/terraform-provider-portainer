package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource              = (*cloudProvisionResource)(nil)
	_ resource.ResourceWithConfigure = (*cloudProvisionResource)(nil)
)

func newCloudProvisionResource() resource.Resource { return &cloudProvisionResource{} }

type cloudProvisionResource struct{ client *APIClient }

type cloudProvisionModel struct {
	ID            types.String   `tfsdk:"id"`
	CloudProvider types.String   `tfsdk:"cloud_provider"`
	Payload       types.Map      `tfsdk:"payload"`
	Timeouts      timeouts.Value `tfsdk:"timeouts"`
}

func (r *cloudProvisionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_provider_provision"
}

func (r *cloudProvisionResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		DeprecationMessage: "portainer_cloud_provider_provision is deprecated: Portainer Business Edition has the cloud provisioning API " +
			"(POST /api/cloud/{provider}/provision) only in the 2.39 LTS line and in 2.41 to 2.43, not in 2.44 or later.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"cloud_provider": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{stringvalidator.OneOf("civo", "digitalocean", "linode", "amazon", "azure", "gke")},
				Description:   "Cloud provider (civo, digitalocean, linode, amazon, azure, gke)",
			},
			"payload": schema.MapAttribute{
				Required:      true,
				ElementType:   types.StringType,
				PlanModifiers: []planmodifier.Map{mapplanmodifier.RequiresReplace()},
				Description:   "Raw payload with provisioning parameters.",
			},
		},
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{Create: true}),
		},
	}
}

func (r *cloudProvisionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *cloudProvisionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan cloudProvisionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	timeout, diags := plan.Timeouts.Create(ctx, 30*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	payload := map[string]interface{}{}
	for k, v := range stringMapToGo(ctx, plan.Payload, &resp.Diagnostics) {
		payload[k] = v
	}
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/cloud/%s/provision", r.client.Endpoint, plan.CloudProvider.ValueString())
	var result struct {
		ID int `json:"Id"`
	}
	if err := doJSON(ctx, r.client, http.MethodPost, url, payload, &result); err != nil {
		err = explainRemovedAPI(ctx, r.client, err, "portainer_cloud_provider_provision",
			"Portainer Business Edition has the cloud provisioning API only in the 2.39 LTS line and in 2.41 to 2.43")
		resp.Diagnostics.AddError("Cloud provision failed", fmt.Sprintf("cloud provision failed: %s", err))
		return
	}
	plan.ID = types.StringValue(strconv.Itoa(result.ID))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read keeps the state: the provisioned environment is not tracked here.
func (r *cloudProvisionResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {
}

// Update only records new timeouts: every other attribute requires
// replacement.
func (r *cloudProvisionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan cloudProvisionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete only forgets the provisioning request.
func (r *cloudProvisionResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}
