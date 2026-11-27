package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
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

func newKubernetesStorageClassesDataSource() datasource.DataSource {
	return newK8sDataSource("kubernetes_storage_classes", schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint_id": k8sDSRequiredInt("Identifier of the Kubernetes environment to query."),
			// Computed attributes
			"storage_classes": k8sDSObjectList("The storage classes the cluster offers.",
				k8sDSField{"name", types.StringType, "Name of the storage class."},
				k8sDSField{"provisioner", types.StringType, "Provisioner backing the storage class."},
				k8sDSField{"reclaim_policy", types.StringType, "What happens to a volume when its claim is released, for example `Delete` or `Retain`."},
				k8sDSField{"is_default", types.BoolType, "Whether this is the cluster's default storage class."},
				k8sDSField{"allow_volume_expansion", types.BoolType, "Whether volumes of this class can be grown after they are created."},
				k8sDSField{"mount_options", k8sDSStringList, "Mount options applied to volumes of this class."},
				k8sDSField{"parameters", k8sDSStringMap, "Provisioner-specific parameters of the class."},
				k8sDSField{"labels", k8sDSStringMap, "Labels on the storage class."},
				k8sDSField{"annotations", k8sDSStringMap, "Annotations on the storage class."},
				k8sDSField{"creation_date", types.StringType, "When the storage class was created."},
			),
		},
	}, nil, kubernetesStorageClassesRead)
}

func kubernetesStorageClassesRead(ctx context.Context, client *APIClient, d *k8sDSData) error {
	endpointID := d.Get("endpoint_id").(int)

	var classes []k8sStorageClass
	listURL := fmt.Sprintf("%s/kubernetes/%d/storage_classes", client.Endpoint, endpointID)
	if err := doJSON(ctx, client, http.MethodGet, listURL, nil, &classes); err != nil {
		return fmt.Errorf("failed to list the storage classes of environment %d: %w", endpointID, err)
	}

	entries := make([]interface{}, 0, len(classes))
	for _, class := range classes {
		entries = append(entries, class.fields())
	}

	if err := d.SetFields(map[string]interface{}{"storage_classes": entries}); err != nil {
		return err
	}
	d.SetID(fmt.Sprintf("portainer-kubernetes-storage-classes-%d", endpointID))
	return nil
}

func newKubernetesStorageClassDataSource() datasource.DataSource {
	return newK8sDataSource("kubernetes_storage_class", schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint_id": k8sDSRequiredInt("Identifier of the Kubernetes environment to query."),
			"name":        k8sDSRequiredString("Name of the storage class to inspect."),
			// Computed attributes
			// Not "provisioner": that is a reserved attribute name at the top
			// level of a Terraform block. Nested inside the listing in
			// portainer_kubernetes_storage_classes the plain name is fine.
			"provisioner_name":       k8sDSComputedString("Provisioner backing the storage class."),
			"reclaim_policy":         k8sDSComputedString("What happens to a volume when its claim is released, for example `Delete` or `Retain`."),
			"is_default":             k8sDSComputedBool("Whether this is the cluster's default storage class."),
			"allow_volume_expansion": k8sDSComputedBool("Whether volumes of this class can be grown after they are created."),
			"mount_options":          k8sDSComputedStringList("Mount options applied to volumes of this class."),
			"parameters":             k8sDSComputedStringMap("Provisioner-specific parameters of the class."),
			"labels":                 k8sDSComputedStringMap("Labels on the storage class."),
			"annotations":            k8sDSComputedStringMap("Annotations on the storage class."),
			"creation_date":          k8sDSComputedString("When the storage class was created."),
		},
	}, nil, kubernetesStorageClassRead)
}

func kubernetesStorageClassRead(ctx context.Context, client *APIClient, d *k8sDSData) error {
	endpointID := d.Get("endpoint_id").(int)
	name := d.Get("name").(string)

	var class k8sStorageClass
	classURL := fmt.Sprintf("%s/kubernetes/%d/storage_classes/%s", client.Endpoint, endpointID, url.PathEscape(name))
	if err := doJSON(ctx, client, http.MethodGet, classURL, nil, &class); err != nil {
		return fmt.Errorf("failed to read storage class %s: %w", name, err)
	}

	fields := class.fields()
	// The name is an argument, so a response is never allowed to rewrite it.
	delete(fields, "name")
	// See the schema: the top-level attribute cannot be called "provisioner".
	fields["provisioner_name"] = fields["provisioner"]
	delete(fields, "provisioner")
	if err := d.SetFields(fields); err != nil {
		return err
	}
	d.SetID(fmt.Sprintf("portainer-kubernetes-storage-class-%d-%s", endpointID, name))
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

func newKubernetesPersistentVolumeClaimsDataSource() datasource.DataSource {
	return newK8sDataSource("kubernetes_persistent_volume_claims", schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint_id": k8sDSRequiredInt("Identifier of the Kubernetes environment to query."),
			// Portainer has a separate path for a single namespace; leaving
			// this unset lists the whole cluster.
			"namespace": k8sDSOptionalString("Only list claims in this namespace. Leave unset to list every claim in the cluster."),
			// Computed attributes
			"claims": k8sDSObjectList("The persistent volume claims.",
				k8sDSField{"id", types.StringType, "Kubernetes UID of the claim."},
				k8sDSField{"name", types.StringType, "Name of the claim."},
				k8sDSField{"namespace", types.StringType, "Namespace of the claim."},
				k8sDSField{"phase", types.StringType, "Phase of the claim, for example `Bound` or `Pending`. A claim stuck in `Pending` is the one worth looking at."},
				k8sDSField{"storage_class", types.StringType, "Storage class the claim asks for."},
				k8sDSField{"storage_request", types.StringType, "Requested size as Kubernetes spells it, for example `10Gi`."},
				k8sDSField{"storage", types.Int64Type, "Requested size in bytes."},
				k8sDSField{"volume_name", types.StringType, "Persistent volume the claim is bound to, empty while it is unbound."},
				k8sDSField{"volume_mode", types.StringType, "Volume mode of the claim, for example `Filesystem`."},
				k8sDSField{"access_modes", k8sDSStringList, "Access modes the claim asks for."},
				k8sDSField{"human_readable_access_modes", k8sDSStringList, "The same access modes, spelled out for display."},
				k8sDSField{"owning_applications", k8sDSStringList, "Applications using the claim. An empty list on a bound claim means nothing is mounting it."},
				k8sDSField{"labels", k8sDSStringMap, "Labels on the claim."},
				k8sDSField{"creation_date", types.StringType, "When the claim was created."},
			),
		},
	}, nil, kubernetesPersistentVolumeClaimsRead)
}

func kubernetesPersistentVolumeClaimsRead(ctx context.Context, client *APIClient, d *k8sDSData) error {
	endpointID := d.Get("endpoint_id").(int)
	namespace := d.Get("namespace").(string)

	listURL := fmt.Sprintf("%s/kubernetes/%d/persistent_volume_claims", client.Endpoint, endpointID)
	if namespace != "" {
		listURL = fmt.Sprintf("%s/kubernetes/%d/namespaces/%s/persistent_volume_claims",
			client.Endpoint, endpointID, url.PathEscape(namespace))
	}

	var claims []k8sPersistentVolumeClaim
	if err := doJSON(ctx, client, http.MethodGet, listURL, nil, &claims); err != nil {
		return fmt.Errorf("failed to list the persistent volume claims of environment %d: %w", endpointID, err)
	}

	entries := make([]interface{}, 0, len(claims))
	for _, claim := range claims {
		entries = append(entries, claim.fields())
	}

	if err := d.SetFields(map[string]interface{}{"claims": entries}); err != nil {
		return err
	}
	d.SetID(fmt.Sprintf("portainer-kubernetes-pvcs-%d-%s", endpointID, namespace))
	return nil
}

func newKubernetesPersistentVolumeClaimDataSource() datasource.DataSource {
	return newK8sDataSource("kubernetes_persistent_volume_claim", schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint_id": k8sDSRequiredInt("Identifier of the Kubernetes environment to query."),
			"namespace":   k8sDSRequiredString("Namespace of the claim."),
			"name":        k8sDSRequiredString("Name of the claim to inspect."),
			// Computed attributes
			// v3.0.0 declared "id" with this description; the value Terraform
			// stores is the data source ID set below, as it always was.
			"id":                          k8sDSComputedString("Kubernetes UID of the claim."),
			"phase":                       k8sDSComputedString("Phase of the claim, for example `Bound` or `Pending`."),
			"storage_class":               k8sDSComputedString("Storage class the claim asks for."),
			"storage_request":             k8sDSComputedString("Requested size as Kubernetes spells it, for example `10Gi`."),
			"storage":                     k8sDSComputedInt("Requested size in bytes."),
			"volume_name":                 k8sDSComputedString("Persistent volume the claim is bound to, empty while it is unbound."),
			"volume_mode":                 k8sDSComputedString("Volume mode of the claim, for example `Filesystem`."),
			"access_modes":                k8sDSComputedStringList("Access modes the claim asks for."),
			"human_readable_access_modes": k8sDSComputedStringList("The same access modes, spelled out for display."),
			"owning_applications":         k8sDSComputedStringList("Applications using the claim."),
			"labels":                      k8sDSComputedStringMap("Labels on the claim."),
			"creation_date":               k8sDSComputedString("When the claim was created."),
		},
	}, nil, kubernetesPersistentVolumeClaimRead)
}

func kubernetesPersistentVolumeClaimRead(ctx context.Context, client *APIClient, d *k8sDSData) error {
	endpointID := d.Get("endpoint_id").(int)
	namespace := d.Get("namespace").(string)
	name := d.Get("name").(string)

	var claim k8sPersistentVolumeClaim
	claimURL := fmt.Sprintf("%s/kubernetes/%d/namespaces/%s/persistent_volume_claims/%s",
		client.Endpoint, endpointID, url.PathEscape(namespace), url.PathEscape(name))
	if err := doJSON(ctx, client, http.MethodGet, claimURL, nil, &claim); err != nil {
		return fmt.Errorf("failed to read persistent volume claim %s/%s: %w", namespace, name, err)
	}

	fields := claim.fields()
	// name and namespace are arguments, so a response never rewrites them.
	delete(fields, "name")
	delete(fields, "namespace")
	if err := d.SetFields(fields); err != nil {
		return err
	}
	d.SetID(fmt.Sprintf("portainer-kubernetes-pvc-%d-%s-%s", endpointID, namespace, name))
	return nil
}

func newKubernetesVolumesDataSource() datasource.DataSource {
	return newK8sDataSource("kubernetes_volumes", schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint_id":       k8sDSRequiredInt("Identifier of the Kubernetes environment to query."),
			"namespace":         k8sDSOptionalString("Only list volumes in this namespace. Leave unset to list every volume in the cluster."),
			"with_applications": k8sDSOptionalBoolDefault("Whether to include the applications using each volume, which costs Portainer an extra lookup."),
			// Computed attributes
			// Portainer does not give this response a fixed shape in its own
			// specification, so it is carried through as JSON rather than
			// flattened into attributes that could go stale.
			"volumes": k8sDSComputedString("The volumes as Portainer returns them, encoded as JSON. Decode it with `jsondecode()`. Use `portainer_kubernetes_volume` for a typed view of one volume."),
		},
	}, map[string]interface{}{"with_applications": false}, kubernetesVolumesRead)
}

func kubernetesVolumesRead(ctx context.Context, client *APIClient, d *k8sDSData) error {
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
		return fmt.Errorf("failed to list the volumes of environment %d: %w", endpointID, err)
	}

	if err := d.SetFields(map[string]interface{}{"volumes": string(body)}); err != nil {
		return err
	}
	d.SetID(fmt.Sprintf("portainer-kubernetes-volumes-%d-%s", endpointID, namespace))
	return nil
}

func newKubernetesVolumeDataSource() datasource.DataSource {
	return newK8sDataSource("kubernetes_volume", schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint_id": k8sDSRequiredInt("Identifier of the Kubernetes environment to query."),
			"namespace":   k8sDSRequiredString("Namespace of the volume."),
			"volume":      k8sDSRequiredString("Name of the volume to inspect."),
			// Computed attributes
			"persistent_volume_name":           k8sDSComputedString("Name of the persistent volume behind the claim."),
			"persistent_volume_status":         k8sDSComputedString("Phase of the persistent volume, for example `Bound`."),
			"persistent_volume_reclaim_policy": k8sDSComputedString("What happens to the volume when its claim is released."),
			"claim_name":                       k8sDSComputedString("Name of the claim bound to the volume."),
			"claim_phase":                      k8sDSComputedString("Phase of the claim, for example `Bound` or `Pending`."),
			"claim_storage_request":            k8sDSComputedString("Size the claim asks for, as Kubernetes spells it."),
			"storage_class_name":               k8sDSComputedString("Storage class backing the volume."),
			"storage_class_provisioner":        k8sDSComputedString("Provisioner of that storage class."),
			// The full response nests CSI and object-reference structures that
			// are too deep and too Kubernetes-version-specific to flatten
			// usefully; the fields above are the ones worth acting on, and
			// this carries the rest.
			"details": k8sDSComputedString("The whole response as Portainer returns it, encoded as JSON. Decode it with `jsondecode()` for anything the attributes above do not cover."),
		},
	}, nil, kubernetesVolumeRead)
}

func kubernetesVolumeRead(ctx context.Context, client *APIClient, d *k8sDSData) error {
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
		return fmt.Errorf("failed to read volume %s/%s: %w", namespace, volume, err)
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return fmt.Errorf("failed to parse volume %s/%s: %w", namespace, volume, err)
	}

	if err := d.SetFields(map[string]interface{}{
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
		return err
	}
	d.SetID(fmt.Sprintf("portainer-kubernetes-volume-%d-%s-%s", endpointID, namespace, volume))
	return nil
}
