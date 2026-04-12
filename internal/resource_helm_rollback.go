package internal

import (
	"fmt"
	"io"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceHelmRollback() *schema.Resource {
	return &schema.Resource{
		Create: resourceHelmRollbackCreate,
		Read:   schema.Noop,
		Delete: schema.Noop,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Environment (Endpoint) identifier",
			},
			"release_name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of the Helm release to rollback",
			},
			"namespace": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "Kubernetes namespace of the release",
			},
			"revision": {
				Type:        schema.TypeInt,
				Optional:    true,
				ForceNew:    true,
				Description: "Revision number to rollback to (defaults to previous revision if not specified)",
			},
			"wait": {
				Type:        schema.TypeBool,
				Optional:    true,
				ForceNew:    true,
				Default:     false,
				Description: "Wait for resources to be ready",
			},
			"wait_for_jobs": {
				Type:        schema.TypeBool,
				Optional:    true,
				ForceNew:    true,
				Default:     false,
				Description: "Wait for jobs to complete before marking the release as successful",
			},
			"recreate": {
				Type:        schema.TypeBool,
				Optional:    true,
				ForceNew:    true,
				Default:     true,
				Description: "Perform pods restart for the resource if applicable",
			},
			"force": {
				Type:        schema.TypeBool,
				Optional:    true,
				ForceNew:    true,
				Default:     false,
				Description: "Force resource update through delete/recreate if needed",
			},
			"timeout": {
				Type:        schema.TypeInt,
				Optional:    true,
				ForceNew:    true,
				Default:     300,
				Description: "Time to wait for any individual Kubernetes operation in seconds",
			},
		},
	}
}

func resourceHelmRollbackCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	releaseName := d.Get("release_name").(string)

	path := fmt.Sprintf("/endpoints/%d/kubernetes/helm/%s/rollback", endpointID, releaseName)

	// Build query parameters
	queryParams := ""
	separator := "?"

	if v, ok := d.GetOk("namespace"); ok {
		queryParams += separator + "namespace=" + v.(string)
		separator = "&"
	}
	if v, ok := d.GetOk("revision"); ok {
		queryParams += separator + "revision=" + strconv.Itoa(v.(int))
		separator = "&"
	}
	if v, ok := d.GetOk("wait"); ok && v.(bool) {
		queryParams += separator + "wait=true"
		separator = "&"
	}
	if v, ok := d.GetOk("wait_for_jobs"); ok && v.(bool) {
		queryParams += separator + "waitForJobs=true"
		separator = "&"
	}
	if v, ok := d.GetOk("recreate"); ok {
		queryParams += separator + "recreate=" + strconv.FormatBool(v.(bool))
		separator = "&"
	}
	if v, ok := d.GetOk("force"); ok && v.(bool) {
		queryParams += separator + "force=true"
		separator = "&"
	}
	if v, ok := d.GetOk("timeout"); ok {
		queryParams += separator + "timeout=" + strconv.Itoa(v.(int))
	}

	resp, err := client.DoRequest("POST", path+queryParams, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to rollback Helm release %s: %w", releaseName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to rollback Helm release %s (status %d): %s", releaseName, resp.StatusCode, string(data))
	}

	d.SetId(fmt.Sprintf("helm-rollback-%d-%s-%d", endpointID, releaseName, makeTimestamp()))
	return nil
}
