package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceResourceControl() *schema.Resource {
	return &schema.Resource{
		Create: resourceResourceControlCreate,
		Read:   resourceResourceControlRead,
		Update: resourceResourceControlUpdate,
		Delete: resourceResourceControlDelete,

		Schema: map[string]*schema.Schema{
			"resource_id": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"resource_control_id": {
				Type:     schema.TypeInt,
				Optional: true,
				ForceNew: true,
			},
			"type": {
				Type:     schema.TypeInt,
				Optional: true,
				ForceNew: true,
				Default:  6,
			},
			"administrators_only": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"public": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"teams": {
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeInt},
			},
			"users": {
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeInt},
			},
		},
	}
}

func lookupResourceControlID(client *APIClient, resourceType int, resourceId string) (string, map[string]interface{}, error) {
	switch resourceType {
	case 6: // stack
		resp, err := client.DoRequest("GET", fmt.Sprintf("/stacks/%s", resourceId), nil, nil)
		if err != nil {
			return "", nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			data, _ := io.ReadAll(resp.Body)
			return "", nil, fmt.Errorf("failed to lookup stack: %s", string(data))
		}

		var result struct {
			ResourceControl map[string]interface{} `json:"ResourceControl"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return "", nil, err
		}
		if result.ResourceControl == nil || result.ResourceControl["Id"] == nil {
			return "", nil, fmt.Errorf("no resource control found for stack %s", resourceId)
		}

		id := int(result.ResourceControl["Id"].(float64))
		return strconv.Itoa(id), result.ResourceControl, nil

	default:
		return "", nil, fmt.Errorf("unsupported resource type: %d", resourceType)
	}
}

func resourceResourceControlRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	// 1) Pokud máme přímo resource_control_id (třeba z docker_secret),
	//    nevoláme žádné API, jen nastavíme ID ve state.
	if v, ok := d.GetOk("resource_control_id"); ok && v.(int) != 0 {
		rcInt := v.(int)
		rcId := strconv.Itoa(rcInt)

		// Nastavíme ID resource v TF
		d.SetId(rcId)

		// Pro jistotu uložíme zpět i resource_control_id,
		// kdyby přišlo z importu nebo staršího state.
		_ = d.Set("resource_control_id", rcInt)

		// Ostatní atributy (administrators_only, public, teams, users)
		// necháme tak, jak jsou – pochází z konfigurace / předchozího apply.
		return nil
	}

	// 2) Jinak starý režim: lookup podle type + resource_id (stack apod.)
	resourceType := d.Get("type").(int)
	resourceId := d.Get("resource_id").(string)

	rcId, rcData, err := lookupResourceControlID(client, resourceType, resourceId)
	if err != nil {
		d.SetId("")
		return nil
	}

	d.SetId(rcId)

	// uložíme resource_control_id, pokud ho server vrátí
	if v, ok := rcData["Id"].(float64); ok {
		_ = d.Set("resource_control_id", int(v))
	}

	if v, ok := rcData["AdministratorsOnly"].(bool); ok {
		_ = d.Set("administrators_only", v)
	}
	if v, ok := rcData["Public"].(bool); ok {
		_ = d.Set("public", v)
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
		_ = d.Set("teams", teams)
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
		_ = d.Set("users", users)
	}

	return nil
}

func resourceResourceControlCreate(d *schema.ResourceData, meta interface{}) error {
	return resourceResourceControlUpdate(d, meta)
}

func resourceResourceControlUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	var rcId string
	if v, ok := d.GetOk("resource_control_id"); ok && v.(int) != 0 {
		rcId = strconv.Itoa(v.(int))
	} else {
		resourceType := d.Get("type").(int)
		resourceId := d.Get("resource_id").(string)

		var err error
		rcId, _, err = lookupResourceControlID(client, resourceType, resourceId)
		if err != nil {
			return err
		}
	}

	body := map[string]interface{}{
		"administratorsOnly": d.Get("administrators_only").(bool),
		"public":             d.Get("public").(bool),
		"teams":              d.Get("teams"),
		"users":              d.Get("users"),
	}

	resp, err := client.DoRequest("PUT", fmt.Sprintf("/resource_controls/%s", rcId), nil, body)
	if err != nil {
		return fmt.Errorf("failed to update resource control: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update resource control: %s", string(data))
	}

	return resourceResourceControlRead(d, meta)
}

func resourceResourceControlDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	var rcId string
	if v, ok := d.GetOk("resource_control_id"); ok && v.(int) != 0 {
		rcId = strconv.Itoa(v.(int))
	} else {
		resourceType := d.Get("type").(int)
		resourceId := d.Get("resource_id").(string)

		var err error
		rcId, _, err = lookupResourceControlID(client, resourceType, resourceId)
		if err != nil {
			d.SetId("")
			return nil
		}
	}

	resp, err := client.DoRequest("DELETE", fmt.Sprintf("/resource_controls/%s", rcId), nil, nil)
	if err != nil {
		return fmt.Errorf("failed to delete resource control: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode != 404 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete resource control: %s", string(data))
	}

	d.SetId("")
	return nil
}
