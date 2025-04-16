# 🚀 **Resource Documentation: `portainer_portainer_kubernetes_application`**

# portainer_portainer_kubernetes_application

The `portainer_portainer_kubernetes_application` resource allows you to deploy one-off Kubernetes `Application` workloads into a specified namespace on a Kubernetes environment (endpoint) managed via Portainer.

---

## Example Usage
### Create Kubernetes Application from YAML
```hcl
resource "portainer_portainer_kubernetes_application" "example" {
  endpoint_id = 4
  namespace   = "default"
  manifest    = file("${path.module}/application.yaml")
}
```

## Lifecycle & Behavior
The Application is created via the Portainer Kubernetes API.

Any change results in a delete + create.

To update the Application (e.g. name, image), simply modify the manifest and re-apply:

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
| namespace   | string | ✅ yes   | Kubernetes namespace where the Application should be created.    |
| manifest    | string | ✅ yes   | Kubernetes Application manifest (JSON or YAML as a string).      |

---

### Attributes Reference
| Name | Description                               |
|------|-------------------------------------------|
| `id` | 	ID in the format endpoint_id:namespace:application:name    |
