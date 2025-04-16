# 🧾 **Resource Documentation: `portainer_kubernetes_configmaps`**

# portainer_kubernetes_configmaps

The `portainer_kubernetes_configmaps` resource allows you to deploy one-off Kubernetes `Configmaps` workloads into a specified namespace on a Kubernetes environment (endpoint) managed via Portainer.

---

## Example Usage
### Create Kubernetes Configmaps from YAML
```hcl
resource "portainer_kubernetes_configmaps" "example" {
  endpoint_id = 4
  namespace   = "default"
  manifest    = file("${path.module}/configmaps.yaml")
}
```

## Lifecycle & Behavior
The Configmaps is created via the Portainer Kubernetes API.

Any change results in a delete + create.

To update the Configmaps (e.g. name, image), simply modify the manifest and re-apply:

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
| namespace   | string | ✅ yes   | Kubernetes namespace where the Configmaps should be created.    |
| manifest    | string | ✅ yes   | Kubernetes Configmaps manifest (JSON or YAML as a string).      |

---

### Attributes Reference
| Name | Description                               |
|------|-------------------------------------------|
| `id` | 	ID in the format endpoint_id:namespace:configmaps:name    |
