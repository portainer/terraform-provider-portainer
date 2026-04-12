package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceRole() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceRoleRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Filter by role name. If set, only the matching role is returned.",
			},
			"roles": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Role identifier.",
						},
						"name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Role name.",
						},
						"description": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Role description.",
						},
						"priority": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Role priority.",
						},
					},
				},
			},
		},
	}
}

func dataSourceRoleRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	resp, err := client.DoRequest(http.MethodGet, "/roles", nil, nil)
	if err != nil {
		return fmt.Errorf("failed to list roles: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to list roles: HTTP %d", resp.StatusCode)
	}

	var result []struct {
		ID          int    `json:"Id"`
		Name        string `json:"Name"`
		Description string `json:"Description"`
		Priority    int    `json:"Priority"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode roles response: %w", err)
	}

	nameFilter, nameFilterSet := d.GetOk("name")

	roles := make([]map[string]interface{}, 0)
	for _, r := range result {
		if nameFilterSet && r.Name != nameFilter.(string) {
			continue
		}
		roles = append(roles, map[string]interface{}{
			"id":          r.ID,
			"name":        r.Name,
			"description": r.Description,
			"priority":    r.Priority,
		})
	}

	if nameFilterSet && len(roles) == 0 {
		return fmt.Errorf("role with name %q not found", nameFilter.(string))
	}

	_ = d.Set("roles", roles)

	if nameFilterSet && len(roles) == 1 {
		d.SetId(strconv.Itoa(roles[0]["id"].(int)))
	} else {
		d.SetId(strconv.FormatInt(time.Now().Unix(), 10))
	}

	return nil
}
