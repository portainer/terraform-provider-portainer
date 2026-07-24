package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceHelmRollback() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceHelmRollbackCreate,
		ReadContext:   schema.NoopContext,
		DeleteContext: schema.NoopContext,

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

func resourceHelmRollbackCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
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

	url := client.Endpoint + path + queryParams
	if err := doJSON(ctx, client, http.MethodPost, url, nil, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to rollback Helm release %s: %w", releaseName, err))
	}

	d.SetId(fmt.Sprintf("helm-rollback-%d-%s-%d", endpointID, releaseName, makeTimestamp()))
	return nil
}
