# 🧩 **Resource Documentation: `portainer_kubernetes_storage`**

# portainer_kubernetes_storage

The `portainer_kubernetes_storage` resource allows you to deploy one-off Kubernetes `Storage` workloads into a specified namespace on a Kubernetes environment (endpoint) managed via Portainer.

---

## Example Usage
### Create Kubernetes Storage from YAML
```hcl
resource "portainer_kubernetes_storage" "example" {
  endpoint_id = 4
  manifest    = file("${path.module}/storage.yaml")
}
```

## Lifecycle & Behavior
The Storage is created via the Portainer Kubernetes API.

Any change results in a delete + create.

To update the Storage (e.g. name, image), simply modify the manifest and re-apply:

```sh
terraform apply
```

To remove the Job:
```sh
terraform destroy
```

### Arguments Reference
| Name        | Type   | Required | Description                                                  |
|-------------|--------|----------|--------------------------------------------------------------|
| endpoint_id | int    | ✅ yes   | ID of the Portainer environment (Kubernetes cluster).        |
| manifest    | string | ✅ yes   | Kubernetes Storage manifest (JSON or YAML as a string).      |

---

### Attributes Reference
| Name | Description                               |
|------|-------------------------------------------|
| `id` | 	ID in the format endpoint_id:namespace:storage:name    |
