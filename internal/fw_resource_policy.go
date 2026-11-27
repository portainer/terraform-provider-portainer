package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// policyTypes are the policy types the Portainer API accepts
// (policies.PolicyType).
var policyTypes = []string{
	"rbac-k8s", "rbac-docker",
	"security-k8s", "security-docker",
	"setup-k8s", "setup-docker",
	"registry-k8s", "registry-docker",
	"change-confirmation",
	"cleanup-docker",
	"observability-k8s",
	"pod-security-standards-k8s",
	"network-security-k8s",
	// Added in Portainer 3.0.
	"vulnerability-scanning-k8s",
	"automatic-namespace-provisioning-k8s",
}

var (
	_ resource.Resource                = (*policyResource)(nil)
	_ resource.ResourceWithConfigure   = (*policyResource)(nil)
	_ resource.ResourceWithImportState = (*policyResource)(nil)
)

func newPolicyResource() resource.Resource { return &policyResource{} }

type policyResource struct{ client *APIClient }

type policyResourceModel struct {
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	EnvironmentType   types.String `tfsdk:"environment_type"`
	PolicyType        types.String `tfsdk:"policy_type"`
	EnvironmentGroups types.List   `tfsdk:"environment_groups"`
	Data              types.String `tfsdk:"data"`
	AllowOverride     types.Bool   `tfsdk:"allow_override"`
	CreatedAt         types.String `tfsdk:"created_at"`
	UpdatedAt         types.String `tfsdk:"updated_at"`
}

func (r *policyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_policy"
}

func (r *policyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the policy.",
			},
			"environment_type": schema.StringAttribute{
				Required:    true,
				Description: "Environment type for the policy (kubernetes, docker, podman, swarm).",
				Validators:  []validator.String{stringvalidator.OneOf("kubernetes", "docker", "podman", "swarm")},
			},
			"policy_type": schema.StringAttribute{
				Required:      true,
				Description:   "Policy type (e.g. rbac-k8s, security-docker). `vulnerability-scanning-k8s` and `automatic-namespace-provisioning-k8s` require Portainer Business Edition 3.0 or later.",
				Validators:    []validator.String{stringvalidator.OneOf(policyTypes...)},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"environment_groups": schema.ListAttribute{
				Optional:    true,
				ElementType: types.Int64Type,
				Description: "List of environment group IDs to which the policy applies.",
			},
			"data": schema.StringAttribute{
				Optional:    true,
				Description: "Policy data as a JSON string. Structure depends on policy_type.",
			},
			"allow_override": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether to allow environments to override this policy.",
			},
			"created_at": schema.StringAttribute{
				Computed:    true,
				Description: "Timestamp when the policy was created.",
			},
			"updated_at": schema.StringAttribute{
				Computed:    true,
				Description: "Timestamp when the policy was last updated.",
			},
		},
	}
}

func (r *policyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

// policyPayload builds the body of POST and PUT /policies, as the SDK
// version did: the groups and the data only when set (and the data only
// when it is a JSON object).
func policyPayload(ctx context.Context, m policyResourceModel) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"Name":            m.Name.ValueString(),
		"EnvironmentType": m.EnvironmentType.ValueString(),
		"Type":            m.PolicyType.ValueString(),
		"AllowOverride":   m.AllowOverride.ValueBool(),
	}
	if !m.EnvironmentGroups.IsNull() && !m.EnvironmentGroups.IsUnknown() && len(m.EnvironmentGroups.Elements()) > 0 {
		var groups []int64
		if d := m.EnvironmentGroups.ElementsAs(ctx, &groups, false); d.HasError() {
			return nil, fmt.Errorf("failed to read environment_groups")
		}
		intGroups := make([]int, 0, len(groups))
		for _, g := range groups {
			intGroups = append(intGroups, int(g))
		}
		payload["EnvironmentGroups"] = intGroups
	}
	if v := m.Data.ValueString(); v != "" {
		var dataMap map[string]interface{}
		if err := json.Unmarshal([]byte(v), &dataMap); err == nil {
			payload["Data"] = dataMap
		}
	}
	return payload, nil
}

func (r *policyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan policyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	payload, err := policyPayload(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create policy", err.Error())
		return
	}

	var result struct {
		ID int `json:"Id"`
	}
	if err := doJSON(ctx, r.client, http.MethodPost, r.client.Endpoint+"/policies", payload, &result); err != nil {
		resp.Diagnostics.AddError("Failed to create policy", fmt.Errorf("failed to create policy: %w", err).Error())
		return
	}
	plan.ID = types.StringValue(strconv.Itoa(result.ID))
	// Save the ID first so a failed read still leaves a tracked policy.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), plan.ID)...)

	if !r.refreshAfterWrite(ctx, &plan, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// refreshAfterWrite reads the policy back after a create or an update. The
// configured arguments stay as planned; the timestamps come from the API.
func (r *policyResource) refreshAfterWrite(ctx context.Context, m *policyResourceModel, diags *diag.Diagnostics) bool {
	got, err := readPolicyAPI(ctx, r.client, m.ID.ValueString())
	if err != nil {
		diags.AddError("Failed to read policy", fmt.Errorf("failed to read policy: %w", err).Error())
		return false
	}
	m.CreatedAt = policyStringOr(got.CreatedAt, types.StringNull())
	m.UpdatedAt = policyStringOr(got.UpdatedAt, types.StringNull())
	return true
}

func (r *policyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state policyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	got, err := readPolicyAPI(ctx, r.client, state.ID.ValueString())
	if err != nil {
		if isAPINotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read policy", fmt.Errorf("failed to read policy: %w", err).Error())
		return
	}

	state.Name = policyStringOr(got.Name, state.Name)
	state.EnvironmentType = policyStringOr(got.environmentType(), state.EnvironmentType)
	state.PolicyType = policyStringOr(got.PolicyType, state.PolicyType)
	state.CreatedAt = policyStringOr(got.CreatedAt, state.CreatedAt)
	state.UpdatedAt = policyStringOr(got.UpdatedAt, state.UpdatedAt)
	if got.HasGroups {
		state.EnvironmentGroups = policyInt64ListOrNull(got.EnvironmentGroups, state.EnvironmentGroups)
	}
	if got.Data != nil {
		switch {
		case state.Data.IsNull() && (*got.Data == "{}" || *got.Data == "null"):
			// An empty object for a policy without data is no drift.
		case !state.Data.IsNull() && policyJSONEquivalent(state.Data.ValueString(), *got.Data):
			// Keep the configured spelling of the same document.
		default:
			state.Data = types.StringValue(*got.Data)
		}
	}
	// allow_override is not read back; an imported policy gets the default.
	if state.AllowOverride.IsNull() {
		state.AllowOverride = types.BoolValue(false)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *policyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state policyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID
	payload, err := policyPayload(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update policy", err.Error())
		return
	}
	if err := doJSON(ctx, r.client, http.MethodPut, fmt.Sprintf("%s/policies/%s", r.client.Endpoint, plan.ID.ValueString()), payload, nil); err != nil {
		resp.Diagnostics.AddError("Failed to update policy", fmt.Errorf("failed to update policy: %w", err).Error())
		return
	}
	if !r.refreshAfterWrite(ctx, &plan, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *policyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state policyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := doJSON(ctx, r.client, http.MethodDelete, fmt.Sprintf("%s/policies/%s", r.client.Endpoint, state.ID.ValueString()), nil, nil); err != nil {
		resp.Diagnostics.AddError("Failed to delete policy", fmt.Errorf("failed to delete policy: %w", err).Error())
	}
}

func (r *policyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
