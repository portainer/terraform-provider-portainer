# 🔐 **Resource Documentation: `portainer_kubernetes_secret`**

# portainer_kubernetes_secret

The `portainer_kubernetes_secret` resource allows you to deploy one-off Kubernetes `Secret` workloads into a specified namespace on a Kubernetes environment (endpoint) managed via Portainer.

---

## Example Usage
### Create Kubernetes Secret from YAML
```hcl
resource "portainer_kubernetes_secret" "example" {
  endpoint_id = 4
  namespace   = "default"
  manifest    = file("${path.module}/secret.yaml")
}
```

## Lifecycle & Behavior
The Secret is created via the Portainer Kubernetes API.

Any change results in a delete + create.

To update the Secret (e.g. name, image), simply modify the manifest and re-apply:

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
| namespace   | string | ✅ yes   | Kubernetes namespace where the Secret should be created.    |
| manifest    | string | ✅ yes   | Kubernetes Secret manifest (JSON or YAML as a string).      |

---

### Attributes Reference
| Name | Description                               |
|------|-------------------------------------------|
| `id` | 	ID in the format endpoint_id:namespace:secret:name    |
