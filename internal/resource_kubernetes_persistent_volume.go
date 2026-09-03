package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceKubernetesPersistentVolume() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceKubernetesPersistentVolumeCreate,
		ReadContext:   resourceKubernetesPersistentVolumeRead,
		UpdateContext: resourceKubernetesPersistentVolumeCreate,
		DeleteContext: resourceKubernetesPersistentVolumeDelete,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the Portainer Kubernetes environment the volume belongs to.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of an existing PersistentVolume. Portainer has no API to create one, so this resource adopts a volume created by a manifest, a StorageClass provisioner or a Helm chart.",
			},
			"reclaim_policy": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringInSlice([]string{"Retain", "Delete", "Recycle"}, false),
				Description:  "Reclaim policy applied to the volume: `Retain`, `Delete` or `Recycle`. This is the one property of a bound PersistentVolume Kubernetes allows to be changed in place.",
			},
			// Computed attributes
			"phase": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Lifecycle phase of the volume: `Available`, `Bound`, `Released`, `Failed` or `Pending`.",
			},
			"capacity": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Storage capacity of the volume as a Kubernetes quantity string (for example `10Gi`).",
			},
			"storage_class": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "StorageClass the volume belongs to, empty for a statically provisioned volume.",
			},
			"access_modes": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Access modes the volume supports, such as `ReadWriteOnce` or `ReadWriteMany`.",
			},
			"claim_ref": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "PersistentVolumeClaim bound to this volume as `<namespace>/<name>`, empty while the volume is unbound.",
			},
		},
	}
}

func resourceKubernetesPersistentVolumeCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	envID := d.Get("environment_id").(int)
	name := d.Get("name").(string)

	payload := map[string]interface{}{
		"name":          name,
		"reclaimPolicy": d.Get("reclaim_policy").(string),
	}
	url := fmt.Sprintf("%s/kubernetes/%d/persistent_volumes/reclaim_policy", client.Endpoint, envID)
	if err := doJSON(ctx, client, http.MethodPut, url, payload, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to set the reclaim policy of persistent volume %q: %w", name, err))
	}

	d.SetId(fmt.Sprintf("%d/%s", envID, name))
	return resourceKubernetesPersistentVolumeRead(ctx, d, meta)
}

func resourceKubernetesPersistentVolumeRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	envID := d.Get("environment_id").(int)
	name := d.Get("name").(string)

	var pv struct {
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

	url := fmt.Sprintf("%s/kubernetes/%d/persistent_volumes/%s", client.Endpoint, envID, name)
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &pv); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read persistent volume %q: %w", name, err))
	}

	claimRef := ""
	if pv.Spec.ClaimRef.Name != "" {
		claimRef = pv.Spec.ClaimRef.Namespace + "/" + pv.Spec.ClaimRef.Name
	}
	accessModes := pv.Spec.AccessModes
	if accessModes == nil {
		accessModes = []string{}
	}

	if err := setFields(d, map[string]interface{}{
		"reclaim_policy": pv.Spec.PersistentVolumeReclaimPolicy,
		"phase":          pv.Status.Phase,
		"capacity":       pv.Spec.Capacity["storage"],
		"storage_class":  pv.Spec.StorageClassName,
		"access_modes":   accessModes,
		"claim_ref":      claimRef,
	}); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceKubernetesPersistentVolumeDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	envID := d.Get("environment_id").(int)
	name := d.Get("name").(string)

	// The delete endpoint takes a list, so a single volume is sent as a
	// one-element array.
	url := fmt.Sprintf("%s/kubernetes/%d/persistent_volumes/delete", client.Endpoint, envID)
	if err := doJSON(ctx, client, http.MethodPost, url, []string{name}, nil); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to delete persistent volume %q: %w", name, err))
	}

	d.SetId("")
	return nil
}
