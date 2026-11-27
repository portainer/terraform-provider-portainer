package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// d2kNamespacePrefix is what Portainer puts in front of an instance name to
// get the namespace the d2k install runs in.
const d2kNamespacePrefix = "d2k-"

// d2kInstance is one entry of the POST /endpoints/d2k payload.
type d2kInstance struct {
	Name            string `json:"Name"`
	EnvironmentName string `json:"EnvironmentName,omitempty"`
}

// d2kCreatePayload is the POST /endpoints/d2k body. The endpoint installs a
// batch of instances; the resource always sends exactly one.
type d2kCreatePayload struct {
	ParentEnvironmentID int           `json:"ParentEnvironmentID"`
	Instances           []d2kInstance `json:"Instances"`
	SwarmMode           bool          `json:"SwarmMode"`
	GroupID             int           `json:"GroupID,omitempty"`
	TagIDs              []int         `json:"TagIDs,omitempty"`
}

// d2kCreateResult is the per-instance outcome POST /endpoints/d2k reports:
// an instance that fails does not fail the request, so its Error has to be
// checked on its own.
type d2kCreateResult struct {
	Name        string `json:"Name"`
	Namespace   string `json:"Namespace"`
	Error       string `json:"Error"`
	Environment *struct {
		ID int `json:"Id"`
	} `json:"Environment"`
}

// d2kEndpoint is the part of GET /endpoints/{id} a D2K environment is read
// from. D2K is only included for D2K environments.
type d2kEndpoint struct {
	ID      int     `json:"Id"`
	Name    string  `json:"Name"`
	GroupID int     `json:"GroupId"`
	TagIds  []int64 `json:"TagIds"`
	D2K     *struct {
		Namespace           string `json:"Namespace"`
		ParentEnvironmentID int    `json:"ParentEnvironmentID"`
		SwarmMode           bool   `json:"SwarmMode"`
	} `json:"D2K"`
}

var (
	_ resource.Resource                = (*d2kEnvironmentResource)(nil)
	_ resource.ResourceWithConfigure   = (*d2kEnvironmentResource)(nil)
	_ resource.ResourceWithImportState = (*d2kEnvironmentResource)(nil)
)

func newD2KEnvironmentResource() resource.Resource { return &d2kEnvironmentResource{} }

type d2kEnvironmentResource struct{ client *APIClient }

type d2kEnvironmentModel struct {
	ID                  types.String `tfsdk:"id"`
	ParentEnvironmentID types.Int64  `tfsdk:"parent_environment_id"`
	Name                types.String `tfsdk:"name"`
	EnvironmentName     types.String `tfsdk:"environment_name"`
	SwarmMode           types.Bool   `tfsdk:"swarm_mode"`
	GroupID             types.Int64  `tfsdk:"group_id"`
	TagIDs              types.List   `tfsdk:"tag_ids"`
	Namespace           types.String `tfsdk:"namespace"`
	EnvironmentID       types.Int64  `tfsdk:"environment_id"`
}

func (r *d2kEnvironmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_d2k_environment"
}

func (r *d2kEnvironmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the D2K environment.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"parent_environment_id": schema.Int64Attribute{
				Required:      true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
				Description:   "Identifier of the Kubernetes environment d2k is installed in. It must be connected through an Edge Agent 3.0.0 or later in tunnel (standard) mode.",
			},
			"name": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "Name of the d2k install. It runs in the namespace `d2k-<name>`, which is why it cannot be changed in place.",
			},
			"environment_name": schema.StringAttribute{
				Optional: true,
				// Portainer names the environment after the install when no
				// name is given, so the name it picked is read back.
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Description:   "Name of the D2K environment in Portainer. Defaults to `name`. Can be changed in place.",
			},
			"swarm_mode": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Default:       booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
				Description:   "Whether the install emulates a Docker Swarm manager rather than a standalone Docker engine, which adds services, configs, secrets and Swarm stacks. Fixed at install time.",
			},
			"group_id": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(1),
				Description: "ID of the Portainer endpoint group of the D2K environment. Default is 1 (Unassigned).",
			},
			"tag_ids": schema.ListAttribute{
				ElementType:   types.Int64Type,
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
				Description:   "List of tag IDs to assign to the D2K environment.",
			},
			"namespace": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Description:   "Kubernetes namespace of the parent environment the d2k install runs in.",
			},
			"environment_id": schema.Int64Attribute{
				Computed:      true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
				Description:   "Identifier of the D2K environment as a number (the same value as `id`), for the `endpoint_id` of Docker resources managed through it.",
			},
		},
	}
}

func (r *d2kEnvironmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *d2kEnvironmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan d2kEnvironmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := plan.Name.ValueString()
	tagIDs := int64ListToInts(ctx, plan.TagIDs, &resp.Diagnostics)
	payload := d2kCreatePayload{
		ParentEnvironmentID: int(plan.ParentEnvironmentID.ValueInt64()),
		Instances: []d2kInstance{{
			Name:            name,
			EnvironmentName: plan.EnvironmentName.ValueString(),
		}},
		SwarmMode: plan.SwarmMode.ValueBool(),
		GroupID:   int(plan.GroupID.ValueInt64()),
		TagIDs:    tagIDs,
	}

	var results []d2kCreateResult
	if err := doJSON(ctx, r.client, http.MethodPost, r.client.Endpoint+"/endpoints/d2k", payload, &results); err != nil {
		err = explainUnsupported(ctx, r.client, err, "portainer_d2k_environment", requiresBE30)
		resp.Diagnostics.AddError("Failed to create D2K environment", fmt.Sprintf("failed to create D2K environment: %s", err))
		return
	}

	var result *d2kCreateResult
	for i := range results {
		if results[i].Name == name {
			result = &results[i]
			break
		}
	}
	if result == nil {
		resp.Diagnostics.AddError("Failed to create D2K environment", fmt.Sprintf("failed to create D2K environment %q: Portainer reported no result for it", name))
		return
	}

	created := result.Environment != nil && result.Environment.ID != 0
	if result.Error != "" {
		resp.Diagnostics.AddError("Failed to create D2K environment", fmt.Sprintf("failed to create D2K environment %q: %s", name, result.Error))
		// An environment that was made despite an error is kept in state, so
		// Terraform marks it tainted and the next apply removes it rather
		// than leaving it behind.
		if created {
			edgeKeepCreatedOnError(ctx, req.Plan, strconv.Itoa(result.Environment.ID), &resp.State, &resp.Diagnostics)
		}
		return
	}
	if !created {
		resp.Diagnostics.AddError("Failed to create D2K environment", fmt.Sprintf("failed to create D2K environment %q: Portainer returned no environment", name))
		return
	}

	plan.ID = types.StringValue(strconv.Itoa(result.Environment.ID))
	r.readInto(ctx, &plan, &resp.State, &resp.Diagnostics)
	edgeKeepCreatedOnError(ctx, req.Plan, plan.ID.ValueString(), &resp.State, &resp.Diagnostics)
}

func (r *d2kEnvironmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state d2kEnvironmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.readInto(ctx, &state, &resp.State, &resp.Diagnostics)
}

func (r *d2kEnvironmentResource) readInto(ctx context.Context, m *d2kEnvironmentModel, state *tfsdk.State, diags *diag.Diagnostics) {
	var env d2kEndpoint
	if err := doJSON(ctx, r.client, http.MethodGet, fmt.Sprintf("%s/endpoints/%s", r.client.Endpoint, m.ID.ValueString()), nil, &env); err != nil {
		if isAPINotFound(err) {
			state.RemoveResource(ctx)
			return
		}
		diags.AddError("Failed to read D2K environment", fmt.Sprintf("failed to read D2K environment: %s", err))
		return
	}
	if env.D2K == nil {
		diags.AddError("Failed to read D2K environment", fmt.Sprintf("environment %s is not a D2K environment", m.ID.ValueString()))
		return
	}

	tagIDs := make([]int, 0, len(env.TagIds))
	for _, tid := range env.TagIds {
		tagIDs = append(tagIDs, int(tid))
	}

	m.ParentEnvironmentID = types.Int64Value(int64(env.D2K.ParentEnvironmentID))
	// The install name is not stored on its own: it is the namespace without
	// the prefix Portainer added, which also covers an import.
	m.Name = types.StringValue(strings.TrimPrefix(env.D2K.Namespace, d2kNamespacePrefix))
	m.Namespace = types.StringValue(env.D2K.Namespace)
	m.SwarmMode = types.BoolValue(env.D2K.SwarmMode)
	m.EnvironmentName = types.StringValue(env.Name)
	m.GroupID = types.Int64Value(int64(env.GroupID))
	m.TagIDs = int64ListValue(tagIDs)
	m.EnvironmentID = types.Int64Value(int64(env.ID))
	diags.Append(state.Set(ctx, m)...)
}

// Update changes what PUT /endpoints/{id} can change on a D2K environment:
// its name, group and tags. Everything else is fixed by the d2k install and
// forces a new resource.
func (r *d2kEnvironmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state d2kEnvironmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := environmentUpdateBody{
		Name:    plan.EnvironmentName.ValueString(),
		GroupID: int(plan.GroupID.ValueInt64()),
	}
	// An empty list clears the tags, but only when tag_ids itself changed, so
	// tags assigned outside Terraform survive when the attribute is not set.
	tags := int64ListToInts(ctx, plan.TagIDs, &resp.Diagnostics)
	if len(tags) > 0 || (!plan.TagIDs.IsUnknown() && !edgeIntListsEqual(ctx, plan.TagIDs, state.TagIDs, &resp.Diagnostics)) {
		if tags == nil {
			tags = []int{}
		}
		body.TagIDs = tags
	}

	if err := doJSON(ctx, r.client, http.MethodPut, fmt.Sprintf("%s/endpoints/%s", r.client.Endpoint, plan.ID.ValueString()), body, nil); err != nil {
		resp.Diagnostics.AddError("Failed to update D2K environment", fmt.Sprintf("failed to update D2K environment: %s", err))
		return
	}
	r.readInto(ctx, &plan, &resp.State, &resp.Diagnostics)
}

// Delete removes the D2K environment from Portainer. Portainer leaves the
// d2k install and its workloads running in the cluster.
func (r *d2kEnvironmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state d2kEnvironmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := doJSON(ctx, r.client, http.MethodDelete, fmt.Sprintf("%s/endpoints/%s", r.client.Endpoint, state.ID.ValueString()), nil, nil); err != nil && !isAPINotFound(err) {
		resp.Diagnostics.AddError("Failed to delete D2K environment", fmt.Sprintf("failed to delete D2K environment: %s", err))
	}
}

func (r *d2kEnvironmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
