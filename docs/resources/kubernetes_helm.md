# 🚀 **Resource Documentation: `portainer_kubernetes_helm`**

# portainer_kubernetes_helm
The `portainer_kubernetes_helm` resource allows you to deploy a Helm chart into a Kubernetes environment managed by Portainer.

## Example Usage
```hcl
resource "portainer_kubernetes_helm" "example" {
  environment_id = 4
  chart          = "nginx"
  name           = "my-nginx"
  namespace      = "default"
  repo           = "https://charts.bitnami.com/bitnami"
  values         = ""
}
```

## Lifecycle & Behavior
- Terraform updates the namespace if `owner`, `annotations`, or `resource_quota` change.
- Changing the `name` will trigger a destroy and recreate operation due to API limitations (Portainer does not support renaming namespaces).
- You can use `terraform destroy` to delete the namespace completely.

### Arguments Reference
| Name             | Type   | Required | Description                                                                 |
|------------------|--------|----------|-----------------------------------------------------------------------------|
| `environment_id` | number | ✅ yes   | The ID of the Kubernetes environment (endpoint) in Portainer.              |
| `chart`          | string | ✅ yes   | The name of the Helm chart (e.g. `nginx`, `redis`).                        |
| `name`           | string | ✅ yes   | The name of the Helm release.                                              |
| `namespace`      | string | ✅ yes   | Kubernetes namespace to install the chart into (e.g. `default`).           |
| `repo`           | string | ✅ yes   | The Helm chart repository URL (e.g. `https://charts.bitnami.com/bitnami`).|
| `values`         | string | ❌ no    | Optional YAML values for the chart as raw string.                          |

---

### Attributes Reference

| Name | Description                               |
|------|-------------------------------------------|
| `id` | Unique identifier for the Helm release    |
