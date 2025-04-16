# ⚙️ **Resource Documentation: `portainer_kubernetes_ingress`**

# portainer_kubernetes_ingress

The `portainer_kubernetes_ingress` resource allows you to deploy one-off Kubernetes `Ingress` workloads into a specified namespace on a Kubernetes environment (endpoint) managed via Portainer.

---

## Example Usage
### Create Kubernetes Ingress from YAML
```hcl
resource "portainer_kubernetes_ingress" "example" {
  endpoint_id = 4
  namespace   = "default"
  manifest    = file("${path.module}/ingress.yaml")
}
```

## Lifecycle & Behavior
The Ingress is created via the Portainer Kubernetes API.

Any change results in a delete + create.

To update the Ingress (e.g. name, image), simply modify the manifest and re-apply:

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
| namespace   | string | ✅ yes   | Kubernetes namespace where the Ingress should be created.    |
| manifest    | string | ✅ yes   | Kubernetes Ingress manifest (JSON or YAML as a string).      |

---

### Attributes Reference
| Name | Description                               |
|------|-------------------------------------------|
| `id` | 	ID in the format endpoint_id:namespace:ingress:name    |
