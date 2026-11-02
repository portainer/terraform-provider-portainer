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

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceResourceControl() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceResourceControlCreate,
		ReadContext:   resourceResourceControlRead,
		UpdateContext: resourceResourceControlUpdate,
		DeleteContext: resourceResourceControlDelete,

		Schema: map[string]*schema.Schema{
			"resource_id": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "Identifier of the underlying Portainer resource (e.g. stack ID) the resource control applies to.",
			},
			"resource_control_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				ForceNew:    true,
				Description: "Identifier of an existing Portainer resource control to manage directly instead of looking it up via `resource_id` and `type`.",
			},
			"type": {
				Type:        schema.TypeInt,
				Optional:    true,
				ForceNew:    true,
				Default:     6,
				Description: "Resource type that the control protects (1 = container, 2 = service, 3 = volume, 4 = network, 5 = secret, 6 = stack, 7 = config). Defaults to 6 (stack).",
			},
			"administrators_only": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether the resource is restricted to Portainer administrators only.",
			},
			"public": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether the resource is publicly accessible to all Portainer users.",
			},
			"teams": {
				Type:        schema.TypeList,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "List of Portainer team identifiers granted access to the resource.",
			},
			"users": {
				Type:        schema.TypeList,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "List of Portainer user identifiers granted access to the resource.",
			},
			"endpoint_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				ForceNew:    true,
				Description: "ID of the Docker environment holding the resource. Needed to read back the control of a container, service, volume, network, secret or config (types 1-5 and 7), so changes made in Portainer show up in the plan; without it those controls are tracked as configured.",
			},
			"sub_resource_ids": {
				Type:        schema.TypeList,
				Optional:    true,
				ForceNew:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Identifiers of sub-resources covered by the same control, such as the services and volumes of a stack. Only used when the control is created by this resource.",
			},
		},
	}
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

func resourceResourceControlRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	// 1) With an explicit resource_control_id (e.g. from docker_secret) no API
	//    is called; only the ID is set in state.
	if v, ok := d.GetOk("resource_control_id"); ok && v.(int) != 0 {
		rcInt := v.(int)
		rcID := strconv.Itoa(rcInt)

		// Set the Terraform resource ID
		d.SetId(rcID)

		// Store resource_control_id back as well, in case it came from an
		// import or an older state.
		if err := d.Set("resource_control_id", rcInt); err != nil {
			return diag.FromErr(err)
		}

		// The other attributes (administrators_only, public, teams, users)
		// are left as they are: they come from the configuration or the
		// previous apply.
		return nil
	}

	// 2) Otherwise the original mode: look up by type + resource_id (stack etc.)
	resourceType := d.Get("type").(int)
	resourceID := d.Get("resource_id").(string)

	rcID, rcData, err := lookupResourceControl(ctx, client, resourceType, resourceID, d.Get("endpoint_id").(int))
	if err != nil {
		if errors.Is(err, errResourceControlNotLookupable) && d.Id() != "" {
			// Created through POST /resource_controls: there is no endpoint to
			// read it back, so the attributes stay as configured rather than
			// the resource being dropped from state.
			return nil
		}
		// Only a resource or resource control that is gone leaves state. Any
		// other failure (network, 5xx, auth) used to drop it as well, and the
		// next apply then tried to create a resource control that still
		// existed.
		if isAPINotFound(err) || errors.Is(err, errResourceControlNotFound) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read resource control: %w", err))
	}

	d.SetId(rcID)

	// Note: We intentionally do NOT set resource_control_id here.
	// When using lookup mode (resource_id + type), the resource_control_id
	// may change if Portainer regenerates the resource control internally.
	// Since resource_control_id has ForceNew: true, updating it would cause
	// Terraform to destroy and recreate the resource unnecessarily.
	// The Terraform resource ID (d.SetId) is updated to track the current
	// backend ID, which is sufficient for subsequent Update/Delete operations.

	fields := map[string]interface{}{}
	if v, ok := rcData["AdministratorsOnly"].(bool); ok {
		fields["administrators_only"] = v
	}
	if v, ok := rcData["Public"].(bool); ok {
		fields["public"] = v
	}
	if v, ok := rcData["TeamAccesses"].([]interface{}); ok {
		teams := []int{}
		for _, t := range v {
			if m, ok := t.(map[string]interface{}); ok {
				if tid, ok := m["TeamId"].(float64); ok {
					teams = append(teams, int(tid))
				}
			}
		}
		fields["teams"] = teams
	}
	if v, ok := rcData["UserAccesses"].([]interface{}); ok {
		users := []int{}
		for _, u := range v {
			if m, ok := u.(map[string]interface{}); ok {
				if uid, ok := m["UserId"].(float64); ok {
					users = append(users, int(uid))
				}
			}
		}
		fields["users"] = users
	}

	if err := setFields(d, fields); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceResourceControlCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	// An explicit control ID means the caller is adopting one that already
	// exists, which is the pre-existing behaviour.
	if v, ok := d.GetOk("resource_control_id"); ok && v.(int) != 0 {
		return resourceResourceControlUpdate(ctx, d, meta)
	}

	resourceType := d.Get("type").(int)
	resourceID := d.Get("resource_id").(string)

	// Portainer creates a resource control implicitly for objects deployed
	// through it, so an existing one is updated rather than duplicated.
	if _, _, err := lookupResourceControl(ctx, client, resourceType, resourceID, d.Get("endpoint_id").(int)); err == nil {
		return resourceResourceControlUpdate(ctx, d, meta)
	}

	if resourceID == "" {
		return diag.FromErr(fmt.Errorf("resource_id is required to create a resource control"))
	}

	payload := map[string]interface{}{
		"ResourceID":         resourceID,
		"Type":               resourceType,
		"AdministratorsOnly": d.Get("administrators_only").(bool),
		"Public":             d.Get("public").(bool),
		"Teams":              toIntSlice(d.Get("teams").([]interface{})),
		"Users":              toIntSlice(d.Get("users").([]interface{})),
	}
	if v, ok := d.GetOk("sub_resource_ids"); ok {
		subs := []string{}
		for _, s := range v.([]interface{}) {
			subs = append(subs, s.(string))
		}
		payload["SubResourceIDs"] = subs
	}

	var created struct {
		ID int `json:"Id"`
	}
	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/resource_controls", payload, &created); err != nil {
		return diag.FromErr(fmt.Errorf("failed to create resource control: %w", err))
	}
	if created.ID == 0 {
		return diag.FromErr(fmt.Errorf("failed to create resource control: Portainer returned no identifier"))
	}

	d.SetId(strconv.Itoa(created.ID))
	return resourceResourceControlRead(ctx, d, meta)
}

func resourceResourceControlUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	var rcID string
	if v, ok := d.GetOk("resource_control_id"); ok && v.(int) != 0 {
		rcID = strconv.Itoa(v.(int))
	} else {
		resourceType := d.Get("type").(int)
		resourceID := d.Get("resource_id").(string)

		var err error
		rcID, _, err = lookupResourceControl(ctx, client, resourceType, resourceID, d.Get("endpoint_id").(int))
		if err != nil {
			if !errors.Is(err, errResourceControlNotLookupable) || d.Id() == "" {
				return diag.FromErr(err)
			}
			rcID = d.Id()
		}
	}

	body := map[string]interface{}{
		"administratorsOnly": d.Get("administrators_only").(bool),
		"public":             d.Get("public").(bool),
		"teams":              d.Get("teams"),
		"users":              d.Get("users"),
	}

	if err := doJSON(ctx, client, http.MethodPut, fmt.Sprintf("%s/resource_controls/%s", client.Endpoint, rcID), body, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update resource control: %w", err))
	}

	return resourceResourceControlRead(ctx, d, meta)
}

func resourceResourceControlDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	var rcID string
	if v, ok := d.GetOk("resource_control_id"); ok && v.(int) != 0 {
		rcID = strconv.Itoa(v.(int))
	} else {
		resourceType := d.Get("type").(int)
		resourceID := d.Get("resource_id").(string)

		var err error
		rcID, _, err = lookupResourceControl(ctx, client, resourceType, resourceID, d.Get("endpoint_id").(int))
		if err != nil {
			if isAPINotFound(err) || errors.Is(err, errResourceControlNotFound) {
				d.SetId("")
				return nil
			}
			if !errors.Is(err, errResourceControlNotLookupable) || d.Id() == "" {
				return diag.FromErr(err)
			}
			rcID = d.Id()
		}
	}

	if err := doJSON(ctx, client, http.MethodDelete, fmt.Sprintf("%s/resource_controls/%s", client.Endpoint, rcID), nil, nil); err != nil {
		var se *apiStatusError
		if errors.As(err, &se) && (se.StatusCode == http.StatusNotFound || se.StatusCode == http.StatusForbidden) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to delete resource control: %w", err))
	}

	d.SetId("")
	return nil
}
