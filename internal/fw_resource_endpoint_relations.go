package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource              = (*endpointRelationsResource)(nil)
	_ resource.ResourceWithConfigure = (*endpointRelationsResource)(nil)
)

func newEndpointRelationsResource() resource.Resource { return &endpointRelationsResource{} }

type endpointRelationsResource struct{ client *APIClient }

type endpointRelationsModel struct {
	ID       types.String            `tfsdk:"id"`
	Relation []endpointRelationModel `tfsdk:"relation"`
}

type endpointRelationModel struct {
	EndpointID   types.Int64 `tfsdk:"endpoint_id"`
	EdgeGroupIDs types.List  `tfsdk:"edge_group_ids"`
	TagIDs       types.List  `tfsdk:"tag_ids"`
	GroupID      types.Int64 `tfsdk:"group_id"`
}

func (r *endpointRelationsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_endpoint_relations"
}

func (r *endpointRelationsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of this set of relations.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
		Blocks: map[string]schema.Block{
			"relation": schema.ListNestedBlock{
				Description: "Relations to apply, one block per environment. Portainer applies them in a single transaction, so either all of them land or none do.",
				Validators:  []validator.List{listvalidator.IsRequired(), listvalidator.SizeAtLeast(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"endpoint_id": schema.Int64Attribute{
							Required:    true,
							Description: "Identifier of the environment the relations apply to.",
						},
						"edge_group_ids": schema.ListAttribute{
							Optional:    true,
							ElementType: types.Int64Type,
							Description: "Edge group identifiers the environment belongs to, replacing whatever it had. Omit the attribute to leave its edge groups alone; there is no way to clear them here, because Portainer cannot tell an intentional \"none\" from \"do not touch\".",
						},
						"tag_ids": schema.ListAttribute{
							Optional:    true,
							ElementType: types.Int64Type,
							Description: "Tag identifiers assigned to the environment, replacing whatever it had. Omit the attribute to leave its tags alone; use `portainer_environment` to clear them.",
						},
						"group_id": schema.Int64Attribute{
							Optional:      true,
							Computed:      true,
							PlanModifiers: []planmodifier.Int64{egKeepZeroInt64{}},
							Description:   "Environment group the environment is moved to. Zero, the default, leaves its group untouched — Portainer only applies a non-zero value.",
						},
					},
				},
			},
		},
	}
}

func (r *endpointRelationsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *endpointRelationsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan endpointRelationsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.apply(ctx, plan); err != nil {
		resp.Diagnostics.AddError("Failed to update environment relations", err.Error())
		return
	}
	plan.ID = types.StringValue("endpoint-relations-" + strconv.FormatInt(makeTimestamp(), 10))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *endpointRelationsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state endpointRelationsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.apply(ctx, plan); err != nil {
		resp.Diagnostics.AddError("Failed to update environment relations", err.Error())
		return
	}
	plan.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// apply sends PUT /endpoints/relations.
//
// Relations is keyed by environment ID; JSON object keys are strings, so the
// numeric IDs are formatted as such.
//
// Each field is omitted when it means "leave this alone", because that is
// exactly how Portainer reads the payload: updateRelations acts on Tags and
// EdgeGroups only when they are non-nil, and on Group only when it is
// non-zero. Sending an empty array instead of omitting the key is therefore
// not a no-op — it tells Portainer to CLEAR that environment's tags or edge
// groups. An empty list is treated like an omitted one, as SDKv2 did.
func (r *endpointRelationsResource) apply(ctx context.Context, m endpointRelationsModel) error {
	relations := map[string]interface{}{}
	for _, rel := range m.Relation {
		relation := map[string]interface{}{}
		if ids := egInt64Slice(rel.EdgeGroupIDs); len(ids) > 0 {
			relation["EdgeGroups"] = ids
		}
		if ids := egInt64Slice(rel.TagIDs); len(ids) > 0 {
			relation["Tags"] = ids
		}
		if egInt64Set(rel.GroupID) {
			relation["Group"] = rel.GroupID.ValueInt64()
		}
		relations[strconv.FormatInt(rel.EndpointID.ValueInt64(), 10)] = relation
	}

	payload := map[string]interface{}{"Relations": relations}
	if err := doJSON(ctx, r.client, http.MethodPut, r.client.Endpoint+"/endpoints/relations", payload, nil); err != nil {
		return fmt.Errorf("failed to update environment relations: %w", err)
	}
	return nil
}

// Read changes nothing: Portainer has no endpoint to read the relations back.
func (r *endpointRelationsResource) Read(context.Context, resource.ReadRequest, *resource.ReadResponse) {
}

// Delete only forgets the resource: Portainer has no endpoint to clear
// relations, so the environments stay as they are.
func (r *endpointRelationsResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}
