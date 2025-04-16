package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceKubernetesDeleteObject() *schema.Resource {
	return &schema.Resource{
		Create: resourceKubernetesDeleteObjectCreate,
		Read:   resourceKubernetesDeleteObjectRead,
		Delete: resourceKubernetesDeleteObjectDelete,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},
			"resource_type": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: validation.StringInSlice([]string{
					"cron_jobs", "ingresses", "jobs", "role_bindings",
					"roles", "service_accounts", "services", "cluster_role_bindings",
					"cluster_roles",
				}, false),
			},
			"namespace": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"names": {
				Type:     schema.TypeList,
				Required: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
		},
	}
}

func resourceKubernetesDeleteObjectCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	envID := d.Get("environment_id").(int)
	typePath := d.Get("resource_type").(string)
	namespace := d.Get("namespace").(string)
	nameList := d.Get("names").([]interface{})

	names := make([]string, len(nameList))
	for i, v := range nameList {
		names[i] = v.(string)
	}

	body := map[string][]string{
		namespace: names,
	}

	jsonBody, _ := json.Marshal(body)
	url := fmt.Sprintf("%s/kubernetes/%d/%s/delete", client.Endpoint, envID, typePath)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", client.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete %s: %s", typePath, string(data))
	}

	id := fmt.Sprintf("%d:%s:%s", envID, typePath, strings.Join(names, ","))
	d.SetId(id)
	return nil
}

func resourceKubernetesDeleteObjectRead(d *schema.ResourceData, meta interface{}) error {
	return nil
}

func resourceKubernetesDeleteObjectDelete(d *schema.ResourceData, meta interface{}) error {
	d.SetId("")
	return nil
}
