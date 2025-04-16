# 🌐👤 **Resource Documentation: `portainer_kubernetes_rolebinding`**

# portainer_kubernetes_rolebinding

The `portainer_kubernetes_rolebinding` resource allows you to deploy one-off Kubernetes `Rolebinding` workloads into a specified namespace on a Kubernetes environment (endpoint) managed via Portainer.

---

## Example Usage
### Create Kubernetes Rolebinding from YAML
```hcl
resource "portainer_kubernetes_rolebinding" "example" {
  endpoint_id = 4
  namespace   = "default"
  manifest    = file("${path.module}/rolebinding.yaml")
}
```

## Lifecycle & Behavior
The Rolebinding is created via the Portainer Kubernetes API.

Any change results in a delete + create.

To update the Rolebinding (e.g. name, image), simply modify the manifest and re-apply:

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
| namespace   | string | ✅ yes   | Kubernetes namespace where the Rolebinding should be created.    |
| manifest    | string | ✅ yes   | Kubernetes Rolebinding manifest (JSON or YAML as a string).      |

---

### Attributes Reference
| Name | Description                               |
|------|-------------------------------------------|
| `id` | 	ID in the format endpoint_id:namespace:rolebinding:name    |
