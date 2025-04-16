# 🚀👤 **Resource Documentation: `portainer_kubernetes_service_account`**

# portainer_kubernetes_service_account

The `portainer_kubernetes_service_account` resource allows you to deploy one-off Kubernetes `Service account` workloads into a specified namespace on a Kubernetes environment (endpoint) managed via Portainer.

---

## Example Usage
### Create Kubernetes Service account from YAML
```hcl
resource "portainer_kubernetes_service_account" "example" {
  endpoint_id = 4
  namespace   = "default"
  manifest    = file("${path.module}/serviceaccount.yaml")
}
```

## Lifecycle & Behavior
The Service account is created via the Portainer Kubernetes API.

Any change results in a delete + create.

To update the Service account (e.g. name, image), simply modify the manifest and re-apply:

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
| namespace   | string | ✅ yes   | Kubernetes namespace where the Service account should be created.    |
| manifest    | string | ✅ yes   | Kubernetes Service account manifest (JSON or YAML as a string).      |

---

### Attributes Reference
| Name | Description                               |
|------|-------------------------------------------|
| `id` | 	ID in the format endpoint_id:namespace:serviceaccount:name    |
