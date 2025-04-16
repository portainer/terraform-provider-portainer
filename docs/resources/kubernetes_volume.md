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

Any change results in a delete + create.

To update the Volume (e.g. name, image), simply modify the manifest and re-apply:

```sh
terraform apply
```

To remove the Job:
```sh
terraform destroy
```

> ⚠️ Volume type must match the manifest kind (e.g. use "persistent-volume-claim" for kind: PersistentVolumeClaim)

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
