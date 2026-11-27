package internal

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	neturl "net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource              = (*resourceControlResource)(nil)
	_ resource.ResourceWithConfigure = (*resourceControlResource)(nil)
)

func newResourceControlResource() resource.Resource { return &resourceControlResource{} }

type resourceControlResource struct{ client *APIClient }

type resourceControlModel struct {
	ID                 types.String `tfsdk:"id"`
	ResourceID         types.String `tfsdk:"resource_id"`
	ResourceControlID  types.Int64  `tfsdk:"resource_control_id"`
	Type               types.Int64  `tfsdk:"type"`
	AdministratorsOnly types.Bool   `tfsdk:"administrators_only"`
	Public             types.Bool   `tfsdk:"public"`
	Teams              types.List   `tfsdk:"teams"`
	Users              types.List   `tfsdk:"users"`
	EndpointID         types.Int64  `tfsdk:"endpoint_id"`
	SubResourceIDs     types.List   `tfsdk:"sub_resource_ids"`
}

func (r *resourceControlResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_resource_control"
}

func (r *resourceControlResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the resource control.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"resource_id": schema.StringAttribute{
				Optional:      true,
				Description:   "Identifier of the underlying Portainer resource (e.g. stack ID) the resource control applies to.",
				PlanModifiers: []planmodifier.String{rcRequiresReplaceString()},
			},
			"resource_control_id": schema.Int64Attribute{
				Optional:      true,
				Description:   "Identifier of an existing Portainer resource control to manage directly instead of looking it up via `resource_id` and `type`.",
				PlanModifiers: []planmodifier.Int64{egRequiresReplaceUnlessZero()},
			},
			"type": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Default:       int64default.StaticInt64(6),
				Description:   "Resource type that the control protects (1 = container, 2 = service, 3 = volume, 4 = network, 5 = secret, 6 = stack, 7 = config). Defaults to 6 (stack).",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"administrators_only": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether the resource is restricted to Portainer administrators only.",
			},
			"public": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether the resource is publicly accessible to all Portainer users.",
			},
			"teams": schema.ListAttribute{
				Optional:    true,
				ElementType: types.Int64Type,
				Description: "List of Portainer team identifiers granted access to the resource.",
			},
			"users": schema.ListAttribute{
				Optional:    true,
				ElementType: types.Int64Type,
				Description: "List of Portainer user identifiers granted access to the resource.",
			},
			"endpoint_id": schema.Int64Attribute{
				Optional:      true,
				Description:   "ID of the Docker environment holding the resource. Needed to read back the control of a container, service, volume, network, secret or config (types 1-5 and 7), so changes made in Portainer show up in the plan; without it those controls are tracked as configured.",
				PlanModifiers: []planmodifier.Int64{egRequiresReplaceUnlessZero()},
			},
			"sub_resource_ids": schema.ListAttribute{
				Optional:      true,
				ElementType:   types.StringType,
				Description:   "Identifiers of sub-resources covered by the same control, such as the services and volumes of a stack. Only used when the control is created by this resource.",
				PlanModifiers: []planmodifier.List{rcRequiresReplaceList()},
			},
		},
	}
}

// rcRequiresReplaceString is RequiresReplace for an optional string whose ""
// means "not set", as SDKv2 saw it.
func rcRequiresReplaceString() planmodifier.String {
	return stringplanmodifier.RequiresReplaceIf(
		func(_ context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
			resp.RequiresReplace = req.StateValue.ValueString() != req.PlanValue.ValueString()
		},
		"Changing this value forces resource recreation.",
		"Changing this value forces resource recreation.",
	)
}

// rcRequiresReplaceList is RequiresReplace for an optional list whose empty
// value means "not set", as SDKv2 saw it.
func rcRequiresReplaceList() planmodifier.List {
	return listplanmodifier.RequiresReplaceIf(
		func(_ context.Context, req planmodifier.ListRequest, resp *listplanmodifier.RequiresReplaceIfFuncResponse) {
			if len(req.StateValue.Elements()) == 0 && len(req.PlanValue.Elements()) == 0 {
				return
			}
			resp.RequiresReplace = !req.StateValue.Equal(req.PlanValue)
		},
		"Changing this value forces resource recreation.",
		"Changing this value forces resource recreation.",
	)
}

func (r *resourceControlResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *resourceControlResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan resourceControlModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	m := plan
	m.ID = types.StringValue("")

	// An explicit control ID means the caller is adopting one that already
	// exists, which is the pre-existing behaviour.
	if egInt64Set(plan.ResourceControlID) {
		r.update(ctx, &m, &plan, &resp.State, &resp.Diagnostics)
		return
	}

	resourceType := int(plan.Type.ValueInt64())
	resourceID := plan.ResourceID.ValueString()

	// Portainer creates a resource control implicitly for objects deployed
	// through it, so an existing one is updated rather than duplicated.
	if _, _, err := lookupResourceControl(ctx, r.client, resourceType, resourceID, int(plan.EndpointID.ValueInt64())); err == nil {
		r.update(ctx, &m, &plan, &resp.State, &resp.Diagnostics)
		return
	}

	if resourceID == "" {
		resp.Diagnostics.AddError("Failed to create resource control", "resource_id is required to create a resource control")
		return
	}

	payload := map[string]interface{}{
		"ResourceID":         resourceID,
		"Type":               resourceType,
		"AdministratorsOnly": plan.AdministratorsOnly.ValueBool(),
		"Public":             plan.Public.ValueBool(),
		"Teams":              rcInts(plan.Teams),
		"Users":              rcInts(plan.Users),
	}
	if subs := rcStrings(plan.SubResourceIDs); len(subs) > 0 {
		payload["SubResourceIDs"] = subs
	}

	var created struct {
		ID int `json:"Id"`
	}
	if err := doJSON(ctx, r.client, http.MethodPost, r.client.Endpoint+"/resource_controls", payload, &created); err != nil {
		resp.Diagnostics.AddError("Failed to create resource control", fmt.Errorf("failed to create resource control: %w", err).Error())
		return
	}
	if created.ID == 0 {
		resp.Diagnostics.AddError("Failed to create resource control", "failed to create resource control: Portainer returned no identifier")
		return
	}

	m.ID = types.StringValue(strconv.Itoa(created.ID))
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), m.ID)...)
	found, err := resourceControlRefresh(ctx, r.client, &m)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read resource control", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("Failed to read resource control", fmt.Sprintf("resource control %s was not found right after it was created", m.ID.ValueString()))
		return
	}
	resourceControlFinish(ctx, &m, &plan, &resp.State, &resp.Diagnostics)
}

func (r *resourceControlResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state resourceControlModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	found, err := resourceControlRefresh(ctx, r.client, &state)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read resource control", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *resourceControlResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state resourceControlModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	m := plan
	m.ID = state.ID
	r.update(ctx, &m, &plan, &resp.State, &resp.Diagnostics)
}

// update sends PUT /resource_controls/{id} for the control found by ID or by
// lookup, then reads it back, as the SDKv2 Update did.
func (r *resourceControlResource) update(ctx context.Context, m, plan *resourceControlModel, state rcStateSetter, diags *diag.Diagnostics) {
	var rcID string
	if egInt64Set(m.ResourceControlID) {
		rcID = strconv.FormatInt(m.ResourceControlID.ValueInt64(), 10)
	} else {
		var err error
		rcID, _, err = lookupResourceControl(ctx, r.client, int(m.Type.ValueInt64()), m.ResourceID.ValueString(), int(m.EndpointID.ValueInt64()))
		if err != nil {
			if !errors.Is(err, errResourceControlNotLookupable) || m.ID.ValueString() == "" {
				diags.AddError("Failed to update resource control", err.Error())
				return
			}
			rcID = m.ID.ValueString()
		}
	}

	body := map[string]interface{}{
		"administratorsOnly": m.AdministratorsOnly.ValueBool(),
		"public":             m.Public.ValueBool(),
		"teams":              rcInts(m.Teams),
		"users":              rcInts(m.Users),
	}
	if err := doJSON(ctx, r.client, http.MethodPut, fmt.Sprintf("%s/resource_controls/%s", r.client.Endpoint, rcID), body, nil); err != nil {
		diags.AddError("Failed to update resource control", fmt.Errorf("failed to update resource control: %w", err).Error())
		return
	}

	found, err := resourceControlRefresh(ctx, r.client, m)
	if err != nil {
		diags.AddError("Failed to read resource control", err.Error())
		return
	}
	if !found {
		diags.AddError("Failed to read resource control", fmt.Sprintf("resource control %s was not found after the update", rcID))
		return
	}
	resourceControlFinish(ctx, m, plan, state, diags)
}

func (r *resourceControlResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state resourceControlModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var rcID string
	if egInt64Set(state.ResourceControlID) {
		rcID = strconv.FormatInt(state.ResourceControlID.ValueInt64(), 10)
	} else {
		var err error
		rcID, _, err = lookupResourceControl(ctx, r.client, int(state.Type.ValueInt64()), state.ResourceID.ValueString(), int(state.EndpointID.ValueInt64()))
		if err != nil {
			if isAPINotFound(err) || errors.Is(err, errResourceControlNotFound) {
				return
			}
			if !errors.Is(err, errResourceControlNotLookupable) || state.ID.ValueString() == "" {
				resp.Diagnostics.AddError("Failed to delete resource control", err.Error())
				return
			}
			rcID = state.ID.ValueString()
		}
	}

	if err := doJSON(ctx, r.client, http.MethodDelete, fmt.Sprintf("%s/resource_controls/%s", r.client.Endpoint, rcID), nil, nil); err != nil {
		var se *apiStatusError
		if errors.As(err, &se) && (se.StatusCode == http.StatusNotFound || se.StatusCode == http.StatusForbidden) {
			return
		}
		resp.Diagnostics.AddError("Failed to delete resource control", fmt.Errorf("failed to delete resource control: %w", err).Error())
	}
}

// resourceControlRefresh refreshes m the way the SDKv2 Read did. It reports false when the
// resource or its control is gone.
func resourceControlRefresh(ctx context.Context, client *APIClient, m *resourceControlModel) (bool, error) {
	// 1) With an explicit resource_control_id (e.g. from docker_secret) no API
	//    is called; only the ID is set. The other attributes come from the
	//    configuration or the previous apply.
	if egInt64Set(m.ResourceControlID) {
		m.ID = types.StringValue(strconv.FormatInt(m.ResourceControlID.ValueInt64(), 10))
		return true, nil
	}

	// 2) Otherwise look up by type + resource_id (stack etc.)
	resourceType := int(m.Type.ValueInt64())
	if m.Type.IsNull() {
		resourceType = 6
	}
	rcID, rcData, err := lookupResourceControl(ctx, client, resourceType, m.ResourceID.ValueString(), int(m.EndpointID.ValueInt64()))
	if err != nil {
		if errors.Is(err, errResourceControlNotLookupable) && m.ID.ValueString() != "" {
			// Created through POST /resource_controls: there is no endpoint to
			// read it back, so the attributes stay as configured.
			return true, nil
		}
		// Only a resource or resource control that is gone leaves state; any
		// other failure is reported.
		if isAPINotFound(err) || errors.Is(err, errResourceControlNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read resource control: %w", err)
	}

	// resource_control_id is intentionally not set in lookup mode: Portainer
	// may regenerate the control, and the ID tracks the current one.
	m.ID = types.StringValue(rcID)
	if v, ok := rcData["AdministratorsOnly"].(bool); ok {
		m.AdministratorsOnly = types.BoolValue(v)
	}
	if v, ok := rcData["Public"].(bool); ok {
		m.Public = types.BoolValue(v)
	}
	if v, ok := rcData["TeamAccesses"].([]interface{}); ok {
		m.Teams = rcAccessList(v, "TeamId", m.Teams)
	}
	if v, ok := rcData["UserAccesses"].([]interface{}); ok {
		m.Users = rcAccessList(v, "UserId", m.Users)
	}
	return true, nil
}

// rcAccessList reads the team or user IDs of an access list. No access stays
// null while the prior value is null, as v3.0.0 left it in the state.
func rcAccessList(accesses []interface{}, key string, prior types.List) types.List {
	ids := []int64{}
	for _, a := range accesses {
		if m, ok := a.(map[string]interface{}); ok {
			if id, ok := m[key].(float64); ok {
				ids = append(ids, int64(id))
			}
		}
	}
	return egOptInt64List(ids, prior)
}

type rcStateSetter interface {
	Set(ctx context.Context, val interface{}) diag.Diagnostics
}

// resourceControlFinish stores the result of Create or Update: the planned values Terraform
// requires back unchanged, and the ID found.
func resourceControlFinish(ctx context.Context, m, plan *resourceControlModel, state rcStateSetter, diags *diag.Diagnostics) {
	out := *plan
	if out.ID.IsUnknown() || out.ID.IsNull() {
		out.ID = m.ID
	}
	diags.Append(state.Set(ctx, &out)...)
}

// rcInts reads a list(number) into a non-nil []int, which encodes as [] when
// empty, as SDKv2's d.Get did.
func rcInts(l types.List) []int {
	values := egInt64Slice(l)
	out := make([]int, 0, len(values))
	for _, v := range values {
		out = append(out, int(v))
	}
	return out
}

func rcStrings(l types.List) []string {
	if l.IsNull() || l.IsUnknown() {
		return nil
	}
	var out []string
	for _, e := range l.Elements() {
		if s, ok := e.(types.String); ok {
			out = append(out, s.ValueString())
		}
	}
	return out
}

// errResourceControlNotLookupable marks a resource type Portainer offers no way
// to resolve a resource control for. Portainer has no GET /resource_controls/{id}
// either, so for those types the control is tracked by the Terraform ID alone —
// which is why this has to be told apart from "the resource is gone".
var errResourceControlNotLookupable = errors.New("resource control cannot be looked up for this resource type")

// errResourceControlNotFound marks a resource that exists but has no resource
// control any more, which drops the resource from state like a 404 does.
var errResourceControlNotFound = errors.New("no resource control found")

// dockerResourcePaths are the Docker API collections of the resource control
// types Portainer decorates through its Docker proxy.
var dockerResourcePaths = map[int]string{
	1: "containers",
	2: "services",
	3: "volumes",
	4: "networks",
	5: "secrets",
	7: "configs",
}

func lookupResourceControlID(ctx context.Context, client *APIClient, resourceType int, resourceID string) (string, map[string]interface{}, error) {
	return lookupResourceControl(ctx, client, resourceType, resourceID, 0)
}

// lookupResourceControl finds the resource control of a resource. Portainer
// has no endpoint to read one by ID, so it is read from the object it
// protects: the stack, or for Docker objects the "Portainer" metadata its
// Docker proxy adds to them. endpointID is only needed for Docker objects.
func lookupResourceControl(ctx context.Context, client *APIClient, resourceType int, resourceID string, endpointID int) (string, map[string]interface{}, error) {
	if path, ok := dockerResourcePaths[resourceType]; ok {
		if endpointID == 0 {
			return "", nil, fmt.Errorf("%w: type %d without endpoint_id", errResourceControlNotLookupable, resourceType)
		}
		var object struct {
			// The identifier Portainer keys the control by: the full ID
			// (containers, services, networks, secrets, configs, "ID" for
			// the swarm ones) or, for a volume, "<name>_<docker ID>", which
			// the proxy adds as ResourceID. resource_id may be a name or a
			// short ID instead.
			ID         string `json:"Id"`
			SwarmID    string `json:"ID"`
			ResourceID string `json:"ResourceID"`
			Portainer  struct {
				ResourceControl map[string]interface{} `json:"ResourceControl"`
			} `json:"Portainer"`
		}
		url := fmt.Sprintf("%s/endpoints/%d/docker/%s/%s", client.Endpoint, endpointID, path, resourceID)
		if path == "containers" {
			url += "/json"
		}
		err := doJSON(ctx, client, http.MethodGet, url, nil, &object)
		// A volume control's resource_id is usually Portainer's own key,
		// "<name>_<docker ID>", which Docker does not know; the volume is
		// "<name>".
		if name, _, ok := cutLast(resourceID, "_"); ok && resourceType == 3 && isAPINotFound(err) {
			err = doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/endpoints/%d/docker/volumes/%s", client.Endpoint, endpointID, name), nil, &object)
		}
		if err != nil {
			return "", nil, fmt.Errorf("failed to read %s %s: %w", path, resourceID, err)
		}
		return ownResourceControl(object.Portainer.ResourceControl, resourceType, resourceID, object.ID, object.SwarmID, object.ResourceID)
	}

	switch resourceType {
	case 6: // stack
		// A stack deployed outside Portainer has no stack object to read: its
		// resource control is keyed "<endpointId>_<compose project>", not by a
		// numeric stack ID, and GET /stacks/{id} cannot resolve it. Its
		// containers carry it, though.
		if _, err := strconv.Atoi(resourceID); err != nil {
			return lookupExternalStackResourceControl(ctx, client, resourceID)
		}
		var result struct {
			ResourceControl map[string]interface{} `json:"ResourceControl"`
		}
		if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/stacks/%s", client.Endpoint, resourceID), nil, &result); err != nil {
			return "", nil, fmt.Errorf("failed to lookup stack: %w", err)
		}
		id, ok := result.ResourceControl["Id"].(float64)
		if !ok {
			return "", nil, fmt.Errorf("%w for stack %s", errResourceControlNotFound, resourceID)
		}
		return strconv.Itoa(int(id)), result.ResourceControl, nil

	default:
		return "", nil, fmt.Errorf("%w: %d", errResourceControlNotLookupable, resourceType)
	}
}

// cutLast slices s around the last instance of sep.
func cutLast(s, sep string) (before, after string, found bool) {
	if i := strings.LastIndex(s, sep); i >= 0 {
		return s[:i], s[i+len(sep):], true
	}
	return s, "", false
}

// ownResourceControl returns rc when it is the control of the resource itself.
// The Docker proxy also shows a control a resource inherits - a container gets
// its service's or its stack's - and taking that one for ours would point
// every later update at the stack's control.
//
// The control is keyed by the object's canonical identifier, which differs
// from resource_id when that is a name or a short ID, so the identifiers the
// object reports for itself count as well.
func ownResourceControl(rc map[string]interface{}, resourceType int, resourceID string, canonicalIDs ...string) (string, map[string]interface{}, error) {
	id, ok := rc["Id"].(float64)
	rcType, _ := rc["Type"].(float64)
	rcResource, _ := rc["ResourceId"].(string)
	own := rcResource != "" && (rcResource == resourceID || slices.Contains(canonicalIDs, rcResource))
	if !ok || int(rcType) != resourceType || !own {
		return "", nil, fmt.Errorf("%w for %s", errResourceControlNotFound, resourceID)
	}
	return strconv.Itoa(int(id)), rc, nil
}

// lookupExternalStackResourceControl reads the control of a stack deployed
// outside Portainer ("<endpointId>_<compose project>") from its containers,
// which the Docker proxy decorates with it. A container with a control of its
// own shows that one instead, so the stack's control only counts as gone when
// some container shows no control at all; a project without containers, or
// whose containers all have their own, cannot be read and is tracked as
// configured.
func lookupExternalStackResourceControl(ctx context.Context, client *APIClient, resourceID string) (string, map[string]interface{}, error) {
	endpoint, project, ok := strings.Cut(resourceID, "_")
	endpointID, err := strconv.Atoi(endpoint)
	if !ok || err != nil || project == "" {
		return "", nil, fmt.Errorf("%w: external stack %q", errResourceControlNotLookupable, resourceID)
	}

	filters := fmt.Sprintf(`{"label":["com.docker.compose.project=%s"]}`, project)
	url := fmt.Sprintf("%s/endpoints/%d/docker/containers/json?all=1&filters=%s", client.Endpoint, endpointID, neturl.QueryEscape(filters))
	var containers []struct {
		Portainer struct {
			ResourceControl map[string]interface{} `json:"ResourceControl"`
		} `json:"Portainer"`
	}
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &containers); err != nil {
		return "", nil, fmt.Errorf("failed to list the containers of stack %q: %w", resourceID, err)
	}
	if len(containers) == 0 {
		return "", nil, fmt.Errorf("%w: external stack %q has no containers", errResourceControlNotLookupable, resourceID)
	}
	undecorated := false
	for _, c := range containers {
		if c.Portainer.ResourceControl == nil {
			undecorated = true
			continue
		}
		if id, rc, err := ownResourceControl(c.Portainer.ResourceControl, 6, resourceID); err == nil {
			return id, rc, nil
		}
	}
	if undecorated {
		return "", nil, fmt.Errorf("%w for external stack %s", errResourceControlNotFound, resourceID)
	}
	return "", nil, fmt.Errorf("%w: the containers of external stack %q only show their own controls", errResourceControlNotLookupable, resourceID)
}
