package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func newKubernetesPersistentVolumesDataSource() datasource.DataSource {
	return newK8sDataSource("kubernetes_persistent_volumes", schema.Schema{
		Attributes: map[string]schema.Attribute{
			"environment_id": k8sDSRequiredInt("Environment (endpoint) identifier of the Kubernetes environment to query."),
			// Computed attributes
			"persistent_volumes": k8sDSObjectList("PersistentVolumes in the cluster. Combine with `portainer_kubernetes_persistent_volume` to pin the reclaim policy of the ones that matter.",
				k8sDSField{"name", types.StringType, "Name of the volume."},
				k8sDSField{"phase", types.StringType, "Lifecycle phase: `Available`, `Bound`, `Released`, `Failed` or `Pending`."},
				k8sDSField{"capacity", types.StringType, "Storage capacity as a Kubernetes quantity string."},
				k8sDSField{"storage_class", types.StringType, "StorageClass the volume belongs to, empty when statically provisioned."},
				k8sDSField{"reclaim_policy", types.StringType, "Reclaim policy: `Retain`, `Delete` or `Recycle`."},
				k8sDSField{"access_modes", k8sDSStringList, "Access modes the volume supports."},
				k8sDSField{"claim_ref", types.StringType, "Claim bound to the volume as `<namespace>/<name>`, empty while unbound."},
			),
			"released_count": k8sDSComputedInt("Number of volumes in the `Released` phase — storage whose claim is gone but which `Retain` has kept from being reclaimed."),
		},
	}, nil, kubernetesPersistentVolumesRead)
}

func kubernetesPersistentVolumesRead(ctx context.Context, client *APIClient, d *k8sDSData) error {
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
		return fmt.Errorf("failed to list the persistent volumes of environment %d: %w", envID, err)
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

	if err := d.SetFields(map[string]interface{}{
		"persistent_volumes": volumes,
		"released_count":     released,
	}); err != nil {
		return err
	}

	d.SetID(strconv.Itoa(envID) + "/persistent_volumes")
	return nil
}
