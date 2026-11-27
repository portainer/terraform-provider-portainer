package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource              = (*edgeGenerateKeyResource)(nil)
	_ resource.ResourceWithConfigure = (*edgeGenerateKeyResource)(nil)
)

type GenerateEdgeKeyResponse struct {
	EdgeKey string `json:"edgeKey"`
}

// edgeGenerateKeyResource asks Portainer for a general edge key once, on
// creation. Read does nothing; Delete only forgets the key.
func newEdgeGenerateKeyResource() resource.Resource { return &edgeGenerateKeyResource{} }

type edgeGenerateKeyResource struct{ client *APIClient }

type edgeGenerateKeyModel struct {
	ID      types.String `tfsdk:"id"`
	EdgeKey types.String `tfsdk:"edge_key"`
}

func (r *edgeGenerateKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_endpoints_edge_generate_key"
}

func (r *edgeGenerateKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"edge_key": schema.StringAttribute{
				Computed:      true,
				Sensitive:     true,
				Description:   "The generated general edge key.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *edgeGenerateKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *edgeGenerateKeyResource) Create(ctx context.Context, _ resource.CreateRequest, resp *resource.CreateResponse) {
	var result GenerateEdgeKeyResponse
	if err := doJSON(ctx, r.client, http.MethodPost, fmt.Sprintf("%s/endpoints/edge/generate-key", r.client.Endpoint), map[string]string{"edgeKey": ""}, &result); err != nil {
		resp.Diagnostics.AddError("Failed to generate edge key", fmt.Errorf("failed to generate edge key: %w", err).Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &edgeGenerateKeyModel{
		ID:      types.StringValue("portainer-generated-edge-key"),
		EdgeKey: types.StringValue(result.EdgeKey),
	})...)
}

func (r *edgeGenerateKeyResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {
}

// Update is never called: the resource has no configurable attribute.
func (r *edgeGenerateKeyResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Edge keys cannot be updated", "portainer_endpoints_edge_generate_key has no configurable attribute.")
}

func (r *edgeGenerateKeyResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}
