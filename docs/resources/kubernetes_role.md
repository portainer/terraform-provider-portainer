# 👤 **Resource Documentation: `portainer_kubernetes_role`**

# portainer_kubernetes_role

The `portainer_kubernetes_role` resource allows you to deploy one-off Kubernetes `Role` workloads into a specified namespace on a Kubernetes environment (endpoint) managed via Portainer.

---

## Example Usage
### Create Kubernetes Role from YAML
```hcl
resource "portainer_kubernetes_role" "example" {
  endpoint_id = 4
  namespace   = "default"
  manifest    = file("${path.module}/role.yaml")
}
```

## Lifecycle & Behavior
The Role is created via the Portainer Kubernetes API.

Any change results in a delete + create.

To update the Role (e.g. name, image), simply modify the manifest and re-apply:

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
| namespace   | string | ✅ yes   | Kubernetes namespace where the Role should be created.    |
| manifest    | string | ✅ yes   | Kubernetes Role manifest (JSON or YAML as a string).      |

---

### Attributes Reference
| Name | Description                               |
|------|-------------------------------------------|
| `id` | 	ID in the format endpoint_id:namespace:role:name    |
