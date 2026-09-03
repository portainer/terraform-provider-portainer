package internal

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

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

func lookupResourceControlID(client *APIClient, resourceType int, resourceID string) (string, map[string]interface{}, error) {
	switch resourceType {
	case 6: // stack
		var result struct {
			ResourceControl map[string]interface{} `json:"ResourceControl"`
		}
		if err := doJSON(context.Background(), client, http.MethodGet, fmt.Sprintf("%s/stacks/%s", client.Endpoint, resourceID), nil, &result); err != nil {
			return "", nil, fmt.Errorf("failed to lookup stack: %w", err)
		}
		if result.ResourceControl == nil || result.ResourceControl["Id"] == nil {
			return "", nil, fmt.Errorf("no resource control found for stack %s", resourceID)
		}

		id := int(result.ResourceControl["Id"].(float64))
		return strconv.Itoa(id), result.ResourceControl, nil

	default:
		return "", nil, fmt.Errorf("%w: %d", errResourceControlNotLookupable, resourceType)
	}
}

func resourceResourceControlRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	// 1) Pokud máme přímo resource_control_id (třeba z docker_secret),
	//    nevoláme žádné API, jen nastavíme ID ve state.
	if v, ok := d.GetOk("resource_control_id"); ok && v.(int) != 0 {
		rcInt := v.(int)
		rcID := strconv.Itoa(rcInt)

		// Nastavíme ID resource v TF
		d.SetId(rcID)

		// Pro jistotu uložíme zpět i resource_control_id,
		// kdyby přišlo z importu nebo staršího state.
		if err := d.Set("resource_control_id", rcInt); err != nil {
			return diag.FromErr(err)
		}

		// Ostatní atributy (administrators_only, public, teams, users)
		// necháme tak, jak jsou – pochází z konfigurace / předchozího apply.
		return nil
	}

	// 2) Jinak starý režim: lookup podle type + resource_id (stack apod.)
	resourceType := d.Get("type").(int)
	resourceID := d.Get("resource_id").(string)

	rcID, rcData, err := lookupResourceControlID(client, resourceType, resourceID)
	if err != nil {
		if errors.Is(err, errResourceControlNotLookupable) && d.Id() != "" {
			// Created through POST /resource_controls: there is no endpoint to
			// read it back, so the attributes stay as configured rather than
			// the resource being dropped from state.
			return nil
		}
		d.SetId("") // resource not found, remove from state
		return nil
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
	if _, _, err := lookupResourceControlID(client, resourceType, resourceID); err == nil {
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
		rcID, _, err = lookupResourceControlID(client, resourceType, resourceID)
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
		rcID, _, err = lookupResourceControlID(client, resourceType, resourceID)
		if err != nil {
			if !errors.Is(err, errResourceControlNotLookupable) || d.Id() == "" {
				d.SetId("")
				return nil
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
