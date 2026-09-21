package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// k8sStorageClass is the storage class shape both the list and the inspect
// endpoint return.
type k8sStorageClass struct {
	Name                 string            `json:"name"`
	Provisioner          string            `json:"provisioner"`
	ReclaimPolicy        string            `json:"reclaimPolicy"`
	IsDefault            bool              `json:"isDefault"`
	AllowVolumeExpansion bool              `json:"allowVolumeExpansion"`
	MountOptions         []string          `json:"mountOptions"`
	Parameters           map[string]string `json:"parameters"`
	Labels               map[string]string `json:"labels"`
	Annotations          map[string]string `json:"annotations"`
	CreationDate         string            `json:"creationDate"`
}

// stringMap turns a map of strings into the interface map the schema takes.
func stringMap(source map[string]string) map[string]interface{} {
	out := make(map[string]interface{}, len(source))
	for k, v := range source {
		out[k] = v
	}
	return out
}

func (s k8sStorageClass) fields() map[string]interface{} {
	return map[string]interface{}{
		"name":                   s.Name,
		"provisioner":            s.Provisioner,
		"reclaim_policy":         s.ReclaimPolicy,
		"is_default":             s.IsDefault,
		"allow_volume_expansion": s.AllowVolumeExpansion,
		"mount_options":          s.MountOptions,
		"parameters":             stringMap(s.Parameters),
		"labels":                 stringMap(s.Labels),
		"annotations":            stringMap(s.Annotations),
		"creation_date":          s.CreationDate,
	}
}

func dataSourceKubernetesStorageClasses() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesStorageClassesRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Kubernetes environment to query.",
			},
			// Computed attributes
			"storage_classes": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "The storage classes the cluster offers.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Name of the storage class.",
						},
						"provisioner": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Provisioner backing the storage class.",
						},
						"reclaim_policy": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "What happens to a volume when its claim is released, for example `Delete` or `Retain`.",
						},
						"is_default": {
							Type:        schema.TypeBool,
							Computed:    true,
							Description: "Whether this is the cluster's default storage class.",
						},
						"allow_volume_expansion": {
							Type:        schema.TypeBool,
							Computed:    true,
							Description: "Whether volumes of this class can be grown after they are created.",
						},
						"mount_options": {
							Type:        schema.TypeList,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Mount options applied to volumes of this class.",
						},
						"parameters": {
							Type:        schema.TypeMap,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Provisioner-specific parameters of the class.",
						},
						"labels": {
							Type:        schema.TypeMap,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Labels on the storage class.",
						},
						"annotations": {
							Type:        schema.TypeMap,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Annotations on the storage class.",
						},
						"creation_date": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "When the storage class was created.",
						},
					},
				},
			},
		},
	}
}

func dataSourceKubernetesStorageClassesRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)

	var classes []k8sStorageClass
	listURL := fmt.Sprintf("%s/kubernetes/%d/storage_classes", client.Endpoint, endpointID)
	if err := doJSON(ctx, client, http.MethodGet, listURL, nil, &classes); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list the storage classes of environment %d: %w", endpointID, err))
	}

	entries := make([]interface{}, 0, len(classes))
	for _, class := range classes {
		entries = append(entries, class.fields())
	}

	if err := setFields(d, map[string]interface{}{"storage_classes": entries}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-kubernetes-storage-classes-%d", endpointID))
	return nil
}

func dataSourceKubernetesStorageClass() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesStorageClassRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Kubernetes environment to query.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the storage class to inspect.",
			},
			// Computed attributes
			"provisioner_name": {
				Type:     schema.TypeString,
				Computed: true,
				// Not "provisioner": that is a reserved attribute name at the
				// top level of a Terraform block. Nested inside the listing in
				// portainer_kubernetes_storage_classes the plain name is fine.
				Description: "Provisioner backing the storage class.",
			},
			"reclaim_policy": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "What happens to a volume when its claim is released, for example `Delete` or `Retain`.",
			},
			"is_default": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether this is the cluster's default storage class.",
			},
			"allow_volume_expansion": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether volumes of this class can be grown after they are created.",
			},
			"mount_options": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Mount options applied to volumes of this class.",
			},
			"parameters": {
				Type:        schema.TypeMap,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Provisioner-specific parameters of the class.",
			},
			"labels": {
				Type:        schema.TypeMap,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Labels on the storage class.",
			},
			"annotations": {
				Type:        schema.TypeMap,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Annotations on the storage class.",
			},
			"creation_date": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "When the storage class was created.",
			},
		},
	}
}

func dataSourceKubernetesStorageClassRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	name := d.Get("name").(string)

	var class k8sStorageClass
	classURL := fmt.Sprintf("%s/kubernetes/%d/storage_classes/%s", client.Endpoint, endpointID, url.PathEscape(name))
	if err := doJSON(ctx, client, http.MethodGet, classURL, nil, &class); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read storage class %s: %w", name, err))
	}

	fields := class.fields()
	// The name is an argument, so a response is never allowed to rewrite it.
	delete(fields, "name")
	// See the schema: the top-level attribute cannot be called "provisioner".
	fields["provisioner_name"] = fields["provisioner"]
	delete(fields, "provisioner")
	if err := setFields(d, fields); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-kubernetes-storage-class-%d-%s", endpointID, name))
	return nil
}

// k8sPersistentVolumeClaim is the claim shape the list and inspect endpoints
// return.
type k8sPersistentVolumeClaim struct {
	ID                       string            `json:"id"`
	Name                     string            `json:"name"`
	Namespace                string            `json:"namespace"`
	Phase                    string            `json:"phase"`
	StorageClass             string            `json:"storageClass"`
	StorageRequest           string            `json:"storageRequest"`
	Storage                  int64             `json:"storage"`
	VolumeName               string            `json:"volumeName"`
	VolumeMode               string            `json:"volumeMode"`
	AccessModes              []string          `json:"accessModes"`
	HumanReadableAccessModes []string          `json:"humanReadableAccessModes"`
	OwningApplications       []string          `json:"owningApplications"`
	Labels                   map[string]string `json:"labels"`
	CreationDate             string            `json:"creationDate"`
}

func (c k8sPersistentVolumeClaim) fields() map[string]interface{} {
	return map[string]interface{}{
		"id":                          c.ID,
		"name":                        c.Name,
		"namespace":                   c.Namespace,
		"phase":                       c.Phase,
		"storage_class":               c.StorageClass,
		"storage_request":             c.StorageRequest,
		"storage":                     int(c.Storage),
		"volume_name":                 c.VolumeName,
		"volume_mode":                 c.VolumeMode,
		"access_modes":                c.AccessModes,
		"human_readable_access_modes": c.HumanReadableAccessModes,
		"owning_applications":         c.OwningApplications,
		"labels":                      stringMap(c.Labels),
		"creation_date":               c.CreationDate,
	}
}

func dataSourceKubernetesPersistentVolumeClaims() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesPersistentVolumeClaimsRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Kubernetes environment to query.",
			},
			"namespace": {
				Type:     schema.TypeString,
				Optional: true,
				// Portainer has a separate path for a single namespace; leaving
				// this unset lists the whole cluster.
				Description: "Only list claims in this namespace. Leave unset to list every claim in the cluster.",
			},
			// Computed attributes
			"claims": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "The persistent volume claims.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Kubernetes UID of the claim.",
						},
						"name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Name of the claim.",
						},
						"namespace": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Namespace of the claim.",
						},
						"phase": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Phase of the claim, for example `Bound` or `Pending`. A claim stuck in `Pending` is the one worth looking at.",
						},
						"storage_class": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Storage class the claim asks for.",
						},
						"storage_request": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Requested size as Kubernetes spells it, for example `10Gi`.",
						},
						"storage": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Requested size in bytes.",
						},
						"volume_name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Persistent volume the claim is bound to, empty while it is unbound.",
						},
						"volume_mode": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Volume mode of the claim, for example `Filesystem`.",
						},
						"access_modes": {
							Type:        schema.TypeList,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Access modes the claim asks for.",
						},
						"human_readable_access_modes": {
							Type:        schema.TypeList,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "The same access modes, spelled out for display.",
						},
						"owning_applications": {
							Type:        schema.TypeList,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Applications using the claim. An empty list on a bound claim means nothing is mounting it.",
						},
						"labels": {
							Type:        schema.TypeMap,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Labels on the claim.",
						},
						"creation_date": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "When the claim was created.",
						},
					},
				},
			},
		},
	}
}

func dataSourceKubernetesPersistentVolumeClaimsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	namespace := d.Get("namespace").(string)

	listURL := fmt.Sprintf("%s/kubernetes/%d/persistent_volume_claims", client.Endpoint, endpointID)
	if namespace != "" {
		listURL = fmt.Sprintf("%s/kubernetes/%d/namespaces/%s/persistent_volume_claims",
			client.Endpoint, endpointID, url.PathEscape(namespace))
	}

	var claims []k8sPersistentVolumeClaim
	if err := doJSON(ctx, client, http.MethodGet, listURL, nil, &claims); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list the persistent volume claims of environment %d: %w", endpointID, err))
	}

	entries := make([]interface{}, 0, len(claims))
	for _, claim := range claims {
		entries = append(entries, claim.fields())
	}

	if err := setFields(d, map[string]interface{}{"claims": entries}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-kubernetes-pvcs-%d-%s", endpointID, namespace))
	return nil
}

func dataSourceKubernetesPersistentVolumeClaim() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesPersistentVolumeClaimRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Kubernetes environment to query.",
			},
			"namespace": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Namespace of the claim.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the claim to inspect.",
			},
			// Computed attributes
			"id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Kubernetes UID of the claim.",
			},
			"phase": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Phase of the claim, for example `Bound` or `Pending`.",
			},
			"storage_class": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Storage class the claim asks for.",
			},
			"storage_request": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Requested size as Kubernetes spells it, for example `10Gi`.",
			},
			"storage": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Requested size in bytes.",
			},
			"volume_name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Persistent volume the claim is bound to, empty while it is unbound.",
			},
			"volume_mode": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Volume mode of the claim, for example `Filesystem`.",
			},
			"access_modes": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Access modes the claim asks for.",
			},
			"human_readable_access_modes": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "The same access modes, spelled out for display.",
			},
			"owning_applications": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Applications using the claim.",
			},
			"labels": {
				Type:        schema.TypeMap,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Labels on the claim.",
			},
			"creation_date": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "When the claim was created.",
			},
		},
	}
}

func dataSourceKubernetesPersistentVolumeClaimRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	namespace := d.Get("namespace").(string)
	name := d.Get("name").(string)

	var claim k8sPersistentVolumeClaim
	claimURL := fmt.Sprintf("%s/kubernetes/%d/namespaces/%s/persistent_volume_claims/%s",
		client.Endpoint, endpointID, url.PathEscape(namespace), url.PathEscape(name))
	if err := doJSON(ctx, client, http.MethodGet, claimURL, nil, &claim); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read persistent volume claim %s/%s: %w", namespace, name, err))
	}

	fields := claim.fields()
	// name and namespace are arguments, so a response never rewrites them.
	delete(fields, "name")
	delete(fields, "namespace")
	if err := setFields(d, fields); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-kubernetes-pvc-%d-%s-%s", endpointID, namespace, name))
	return nil
}

func dataSourceKubernetesVolumes() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesVolumesRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Kubernetes environment to query.",
			},
			"namespace": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Only list volumes in this namespace. Leave unset to list every volume in the cluster.",
			},
			"with_applications": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to include the applications using each volume, which costs Portainer an extra lookup.",
			},
			// Computed attributes
			"volumes": {
				Type:     schema.TypeString,
				Computed: true,
				// Portainer does not give this response a fixed shape in its
				// own specification, so it is carried through as JSON rather
				// than flattened into attributes that could go stale.
				Description: "The volumes as Portainer returns them, encoded as JSON. Decode it with `jsondecode()`. Use `portainer_kubernetes_volume` for a typed view of one volume.",
			},
		},
	}
}

func dataSourceKubernetesVolumesRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	namespace := d.Get("namespace").(string)

	listURL := fmt.Sprintf("%s/kubernetes/%d/volumes", client.Endpoint, endpointID)
	if namespace != "" {
		listURL = fmt.Sprintf("%s/kubernetes/%d/namespaces/%s/volumes",
			client.Endpoint, endpointID, url.PathEscape(namespace))
	}
	if d.Get("with_applications").(bool) {
		listURL += "?withApplications=true"
	}

	var body json.RawMessage
	if err := doJSON(ctx, client, http.MethodGet, listURL, nil, &body); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list the volumes of environment %d: %w", endpointID, err))
	}

	if err := setFields(d, map[string]interface{}{"volumes": string(body)}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-kubernetes-volumes-%d-%s", endpointID, namespace))
	return nil
}

func dataSourceKubernetesVolume() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesVolumeRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Kubernetes environment to query.",
			},
			"namespace": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Namespace of the volume.",
			},
			"volume": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the volume to inspect.",
			},
			// Computed attributes
			"persistent_volume_name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Name of the persistent volume behind the claim.",
			},
			"persistent_volume_status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Phase of the persistent volume, for example `Bound`.",
			},
			"persistent_volume_reclaim_policy": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "What happens to the volume when its claim is released.",
			},
			"claim_name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Name of the claim bound to the volume.",
			},
			"claim_phase": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Phase of the claim, for example `Bound` or `Pending`.",
			},
			"claim_storage_request": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Size the claim asks for, as Kubernetes spells it.",
			},
			"storage_class_name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Storage class backing the volume.",
			},
			"storage_class_provisioner": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Provisioner of that storage class.",
			},
			"details": {
				Type:     schema.TypeString,
				Computed: true,
				// The full response nests CSI and object-reference structures
				// that are too deep and too Kubernetes-version-specific to
				// flatten usefully; the fields above are the ones worth acting
				// on, and this carries the rest.
				Description: "The whole response as Portainer returns it, encoded as JSON. Decode it with `jsondecode()` for anything the attributes above do not cover.",
			},
		},
	}
}

func dataSourceKubernetesVolumeRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	namespace := d.Get("namespace").(string)
	volume := d.Get("volume").(string)

	var info struct {
		PersistentVolume struct {
			Name          string `json:"name"`
			Status        string `json:"status"`
			ReclaimPolicy string `json:"persistentVolumeReclaimPolicy"`
		} `json:"persistentVolume"`
		PersistentVolumeClaim struct {
			Name           string `json:"name"`
			Phase          string `json:"phase"`
			StorageRequest string `json:"storageRequest"`
		} `json:"persistentVolumeClaim"`
		StorageClass struct {
			Name        string `json:"name"`
			Provisioner string `json:"provisioner"`
		} `json:"storageClass"`
	}

	volumeURL := fmt.Sprintf("%s/kubernetes/%d/volumes/%s/%s",
		client.Endpoint, endpointID, url.PathEscape(namespace), url.PathEscape(volume))

	// The response is read twice: once into the typed fields above, and once
	// raw so nothing in it is lost to the configuration.
	body, err := apiGETRaw(ctx, client, volumeURL)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to read volume %s/%s: %w", namespace, volume, err))
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return diag.FromErr(fmt.Errorf("failed to parse volume %s/%s: %w", namespace, volume, err))
	}

	if err := setFields(d, map[string]interface{}{
		"persistent_volume_name":           info.PersistentVolume.Name,
		"persistent_volume_status":         info.PersistentVolume.Status,
		"persistent_volume_reclaim_policy": info.PersistentVolume.ReclaimPolicy,
		"claim_name":                       info.PersistentVolumeClaim.Name,
		"claim_phase":                      info.PersistentVolumeClaim.Phase,
		"claim_storage_request":            info.PersistentVolumeClaim.StorageRequest,
		"storage_class_name":               info.StorageClass.Name,
		"storage_class_provisioner":        info.StorageClass.Provisioner,
		"details":                          string(body),
	}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-kubernetes-volume-%d-%s-%s", endpointID, namespace, volume))
	return nil
}
