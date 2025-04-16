# ⚙️🌐👤 **Resource Documentation: `portainer_kubernetes_clusterrolebinding`**

# portainer_kubernetes_clusterrolebinding

The `portainer_kubernetes_clusterrolebinding` resource allows you to deploy one-off Kubernetes `Clusterrolebinding` workloads into a specified namespace on a Kubernetes environment (endpoint) managed via Portainer.

---

## Example Usage
### Create Kubernetes Clusterrolebinding from YAML
```hcl
resource "portainer_kubernetes_clusterrolebinding" "example" {
  endpoint_id = 4
  manifest    = file("${path.module}/clusterrolebinding.yaml")
}
```

## Lifecycle & Behavior
The Clusterrolebinding is created via the Portainer Kubernetes API.

Any change results in a delete + create.

To update the Clusterrolebinding (e.g. name, image), simply modify the manifest and re-apply:

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
| manifest    | string | ✅ yes   | Kubernetes Clusterrolebinding manifest (JSON or YAML as a string).      |

---

### Attributes Reference
| Name | Description                               |
|------|-------------------------------------------|
| `id` | 	ID in the format endpoint_id:namespace:clusterrolebinding:name    |
