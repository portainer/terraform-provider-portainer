package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// resourceAddonAccess owns the whole access configuration of one addon.
// PUT /addons/{id}/access replaces the stored policies wholesale, so a
// per-grant resource would have each instance revoking the others' grants;
// one resource per addon is the only shape that matches the endpoint.
func resourceAddonAccess() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAddonAccessWrite,
		ReadContext:   resourceAddonAccessRead,
		UpdateContext: resourceAddonAccessWrite,
		DeleteContext: resourceAddonAccessDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"addon_id": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Catalog identifier of the addon whose access configuration this resource owns, for example `portal-template`.",
			},
			"enabled": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "Whether the addon is switched on. A disabled addon stays installed but is hidden from users.",
			},
			"user_access": {
				Type:        schema.TypeSet,
				Optional:    true,
				Description: "Users granted access to the addon. The set is authoritative: a user missing from it has no grant.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"user_id": {
							Type:        schema.TypeInt,
							Required:    true,
							Description: "Identifier of the user granted access.",
						},
						"role_id": {
							Type:        schema.TypeInt,
							Required:    true,
							Description: "Identifier of the role the user is granted on the addon.",
						},
					},
				},
			},
			"team_access": {
				Type:        schema.TypeSet,
				Optional:    true,
				Description: "Teams granted access to the addon. The set is authoritative: a team missing from it has no grant.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"team_id": {
							Type:        schema.TypeInt,
							Required:    true,
							Description: "Identifier of the team granted access.",
						},
						"role_id": {
							Type:        schema.TypeInt,
							Required:    true,
							Description: "Identifier of the role the team is granted on the addon.",
						},
					},
				},
			},
		},
	}
}

// accessPoliciesFromSet turns a user_access/team_access block into the
// {"<id>": {"RoleId": n}} object Portainer stores. The result is never nil:
// an empty object is how the API is told to revoke every grant, which is
// exactly what an empty set in the configuration means.
func accessPoliciesFromSet(raw interface{}, idField string) map[string]map[string]int {
	policies := map[string]map[string]int{}
	set, ok := raw.(*schema.Set)
	if !ok {
		return policies
	}
	for _, item := range set.List() {
		entry, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := entry[idField].(int)
		role, _ := entry["role_id"].(int)
		policies[strconv.Itoa(id)] = map[string]int{"RoleId": role}
	}
	return policies
}

// accessPoliciesToSet is the inverse, for reading state back.
func accessPoliciesToSet(policies map[string]map[string]int, idField string) []interface{} {
	out := make([]interface{}, 0, len(policies))
	for idStr, policy := range policies {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			continue
		}
		out = append(out, map[string]interface{}{
			idField:   id,
			"role_id": policy["RoleId"],
		})
	}
	return out
}

func resourceAddonAccessWrite(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	addonID := d.Get("addon_id").(string)

	payload := map[string]interface{}{
		"enabled":            d.Get("enabled").(bool),
		"userAccessPolicies": accessPoliciesFromSet(d.Get("user_access"), "user_id"),
		"teamAccessPolicies": accessPoliciesFromSet(d.Get("team_access"), "team_id"),
	}

	endpoint := addonURL(client, addonID) + "/access"
	if err := doJSON(ctx, client, http.MethodPut, endpoint, payload, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update the access configuration of addon %s: %w", addonID, err))
	}

	d.SetId(addonID)
	return resourceAddonAccessRead(ctx, d, meta)
}

func resourceAddonAccessRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	addon, err := readAddon(ctx, client, d.Id())
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to read addon %s: %w", d.Id(), err))
	}
	if addon == nil {
		d.SetId("")
		return nil
	}

	fields := map[string]interface{}{
		"addon_id": addon.ID,
		"enabled":  addon.Enabled,
	}
	// The policies live on the admin-only record. Read by a token that cannot
	// see it, the grants are simply left as configured rather than wiped from
	// state on the strength of a response that never carried them.
	if r := addon.Record; r != nil {
		fields["user_access"] = accessPoliciesToSet(r.UserAccessPolicies, "user_id")
		fields["team_access"] = accessPoliciesToSet(r.TeamAccessPolicies, "team_id")
	}

	if err := setFields(d, fields); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceAddonAccessDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	// Destroying the resource revokes the grants it created. `enabled` is
	// deliberately left out of the payload: turning the addon off is a
	// different decision than giving up managing who may use it.
	payload := map[string]interface{}{
		"userAccessPolicies": map[string]map[string]int{},
		"teamAccessPolicies": map[string]map[string]int{},
	}

	endpoint := addonURL(client, d.Id()) + "/access"
	if err := doJSON(ctx, client, http.MethodPut, endpoint, payload, nil); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to revoke the access grants of addon %s: %w", d.Id(), err))
	}

	d.SetId("")
	return nil
}
