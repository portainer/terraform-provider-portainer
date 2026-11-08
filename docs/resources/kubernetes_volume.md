# 📦 **Resource Documentation: `portainer_kubernetes_volume`**

# portainer_kubernetes_volume

The `portainer_kubernetes_volume` resource allows you to create and manage various Kubernetes volume types in a specified namespace using the Portainer Kubernetes API.

Supported volume types include:

- `persistent-volume-claim`
- `persistent-volume`
- `volume-attachment`

---

## Example Usage

### Create Persistent Volume Claim (PVC) from YAML
```hcl
resource "portainer_kubernetes_volume" "example_pvc" {
  endpoint_id = 4
  namespace   = "default"
  type        = "persistent-volume-claim"
  manifest    = file("${path.module}/pvc.yaml")
}
```

### Create Persistent Volume Claim (PVC) from YAML
```hcl
resource "portainer_kubernetes_volume" "example_pv" {
  endpoint_id = 4
  type        = "persistent-volume"
  manifest    = file("${path.module}/pv.yaml")
}
```

### Create Persistent Volume Claim (PVC) from YAML
```hcl
resource "portainer_kubernetes_volume" "example_va" {
  endpoint_id = 4
  type        = "volume-attachment"
  manifest    = file("${path.module}/volume-attachment.yaml")
}
```

## Lifecycle & Behavior
The Volume is created via the Portainer Kubernetes API.

A change of `manifest` updates the existing object in place (a JSON merge patch), so a Deployment rolls out and a Service keeps its cluster IP; fields removed from the manifest are removed from the object. The object is deleted and created again only when it cannot be changed in place: another `metadata.name` or kind, another `namespace` or `type`, or a field Kubernetes does not allow to change (for example a Job's pod template or a Deployment's selector).

To update the Volume (e.g. name, image), simply modify the manifest and re-apply:

```sh
terraform apply
```

To remove the volume:
```sh
terraform destroy
```

> ⚠️ Volume type must match the manifest kind (e.g. use "persistent-volume-claim" for kind: PersistentVolumeClaim)

**Drift detection.** On refresh, the fields set in `manifest` are compared with the live object: a value changed outside Terraform shows up in the plan, as the drifted fields with their live values, and the next apply puts the manifest back. Fields Kubernetes adds itself (status, defaults, `resourceVersion`) and canonical quantities (`500m` for `0.5`) are not drift. Fields the manifest does not set are not managed, so a key added outside Terraform is not reported.

### Arguments Reference
| Name         | Type   | Required   | Description                                                                 |
|--------------|--------|------------|-----------------------------------------------------------------------------|
| `endpoint_id`| int    | ✅ yes     | ID of the Portainer Kubernetes environment.                                 |
| `namespace`  | string | 🚫 optional| Kubernetes namespace (required for PVCs, ignored for PVs and attachments).  |
| `type`       | string | ✅ yes     | Type of volume. One of: `persistent-volume-claim`, `persistent-volume`, `volume-attachment`. |
| `manifest`   | string | ✅ yes     | Kubernetes volume manifest (YAML or JSON as a string).                      |

---

### Attributes Reference
| Name | Description                               |
|------|-------------------------------------------|
| `id` | 	ID in the format endpoint_id:namespace:volumee:name    |

## Import

Kubernetes volume resources can be imported using the composite ID `endpointID:namespace:type:name` (namespace is empty for cluster-scoped types like `persistent-volume`):

```shell
terraform import portainer_kubernetes_volume.example 1:default:persistent-volume-claim:my-pvc
```

After import, set the `manifest` field in config to match the live object — Read only confirms the volume exists and restores `endpoint_id`/`namespace`/`type`, it does not reconstruct the manifest from the live resource.
