package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceKubernetesNodeDrain() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceKubernetesNodeDrainCreate,
		ReadContext:   schema.NoopContext,
		DeleteContext: removeFromStateContext,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the Portainer Kubernetes environment the node belongs to.",
			},
			"node_name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of the Kubernetes node to drain.",
			},
			// The defaults below mirror Portainer's own libkubectl.DefaultDrainOptions,
			// so an unset field behaves exactly as omitting it from the API payload.
			"force": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				ForceNew:    true,
				Description: "Whether to allow deletion of standalone pods that are not managed by a controller. Such pods are lost for good, so the drain refuses them unless this is enabled. Mirrors `kubectl drain --force`.",
			},
			"timeout_seconds": {
				Type:         schema.TypeInt,
				Optional:     true,
				Default:      60,
				ForceNew:     true,
				ValidateFunc: validation.IntAtLeast(0),
				Description:  "Overall time in seconds to wait for the drain to complete. Mirrors `kubectl drain --timeout`.",
			},
			"grace_period_seconds": {
				Type:         schema.TypeInt,
				Optional:     true,
				Default:      -1,
				ForceNew:     true,
				ValidateFunc: validation.IntAtLeast(-1),
				Description:  "Termination grace period in seconds to apply to every evicted pod. `-1` keeps each pod's own grace period. Mirrors `kubectl drain --grace-period`.",
			},
			"ignore_daemon_sets": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				ForceNew:    true,
				Description: "Whether to skip DaemonSet-managed pods, which would otherwise block the drain because their controller recreates them immediately. Mirrors `kubectl drain --ignore-daemonsets`.",
			},
			"delete_empty_dir_data": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				ForceNew:    true,
				Description: "Whether to evict pods that use an emptyDir volume, whose data is lost once the pod is deleted. Mirrors `kubectl drain --delete-emptydir-data`.",
			},
			"disable_eviction": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				ForceNew:    true,
				Description: "Whether to delete pods directly instead of going through the eviction API, which ignores any configured PodDisruptionBudget. Mirrors `kubectl drain --disable-eviction`.",
			},
		},
	}
}

func resourceKubernetesNodeDrainCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	envID := d.Get("environment_id").(int)
	nodeName := d.Get("node_name").(string)

	// Every field is sent explicitly. The schema defaults match Portainer's own
	// drain defaults, so this is equivalent to omitting them while keeping the
	// effective options visible in state and in the plan.
	//
	// The keys are PascalCase on purpose: Portainer's drainNodePayload carries no
	// json tags, so the Go field names are the wire names. camelCase would also
	// decode (encoding/json falls back to a case-insensitive match), but the
	// exact names keep this independent of that fallback.
	body := map[string]interface{}{
		"Force":              d.Get("force").(bool),
		"TimeoutSeconds":     d.Get("timeout_seconds").(int),
		"GracePeriodSeconds": d.Get("grace_period_seconds").(int),
		"IgnoreDaemonSets":   d.Get("ignore_daemon_sets").(bool),
		"DeleteEmptyDirData": d.Get("delete_empty_dir_data").(bool),
		"DisableEviction":    d.Get("disable_eviction").(bool),
	}

	url := fmt.Sprintf("%s/kubernetes/%d/nodes/%s/drain", client.Endpoint, envID, nodeName)
	if err := doJSON(ctx, client, http.MethodPost, url, body, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to drain Kubernetes node %q: %w", nodeName, err))
	}

	d.SetId(fmt.Sprintf("%d/%s", envID, nodeName))
	return nil
}
