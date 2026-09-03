package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceKubernetesPersistentVolumes() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesPersistentVolumesRead,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Environment (endpoint) identifier of the Kubernetes environment to query.",
			},
			// Computed attributes
			"persistent_volumes": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "PersistentVolumes in the cluster. Combine with `portainer_kubernetes_persistent_volume` to pin the reclaim policy of the ones that matter.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name":           {Type: schema.TypeString, Computed: true, Description: "Name of the volume."},
						"phase":          {Type: schema.TypeString, Computed: true, Description: "Lifecycle phase: `Available`, `Bound`, `Released`, `Failed` or `Pending`."},
						"capacity":       {Type: schema.TypeString, Computed: true, Description: "Storage capacity as a Kubernetes quantity string."},
						"storage_class":  {Type: schema.TypeString, Computed: true, Description: "StorageClass the volume belongs to, empty when statically provisioned."},
						"reclaim_policy": {Type: schema.TypeString, Computed: true, Description: "Reclaim policy: `Retain`, `Delete` or `Recycle`."},
						"access_modes":   {Type: schema.TypeList, Computed: true, Elem: &schema.Schema{Type: schema.TypeString}, Description: "Access modes the volume supports."},
						"claim_ref":      {Type: schema.TypeString, Computed: true, Description: "Claim bound to the volume as `<namespace>/<name>`, empty while unbound."},
					},
				},
			},
			"released_count": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of volumes in the `Released` phase — storage whose claim is gone but which `Retain` has kept from being reclaimed.",
			},
		},
	}
}

func dataSourceKubernetesPersistentVolumesRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	envID := d.Get("environment_id").(int)

	var list []struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Spec struct {
			Capacity                      map[string]string `json:"capacity"`
			AccessModes                   []string          `json:"accessModes"`
			PersistentVolumeReclaimPolicy string            `json:"persistentVolumeReclaimPolicy"`
			StorageClassName              string            `json:"storageClassName"`
			ClaimRef                      struct {
				Namespace string `json:"namespace"`
				Name      string `json:"name"`
			} `json:"claimRef"`
		} `json:"spec"`
		Status struct {
			Phase string `json:"phase"`
		} `json:"status"`
	}
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/kubernetes/%d/persistent_volumes", client.Endpoint, envID), nil, &list); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list the persistent volumes of environment %d: %w", envID, err))
	}

	volumes := make([]map[string]interface{}, len(list))
	released := 0
	for i, pv := range list {
		claimRef := ""
		if pv.Spec.ClaimRef.Name != "" {
			claimRef = pv.Spec.ClaimRef.Namespace + "/" + pv.Spec.ClaimRef.Name
		}
		modes := pv.Spec.AccessModes
		if modes == nil {
			modes = []string{}
		}
		if pv.Status.Phase == "Released" {
			released++
		}
		volumes[i] = map[string]interface{}{
			"name": pv.Metadata.Name, "phase": pv.Status.Phase,
			"capacity": pv.Spec.Capacity["storage"], "storage_class": pv.Spec.StorageClassName,
			"reclaim_policy": pv.Spec.PersistentVolumeReclaimPolicy,
			"access_modes":   modes, "claim_ref": claimRef,
		}
	}

	if err := setFields(d, map[string]interface{}{
		"persistent_volumes": volumes,
		"released_count":     released,
	}); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(strconv.Itoa(envID) + "/persistent_volumes")
	return nil
}
