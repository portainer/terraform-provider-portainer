# 🧩 **Resource Documentation: `portainer_kubernetes_namespace`**

The `portainer_kubernetes_namespace` resource allows you to manage Kubernetes namespaces within a specific environment (endpoint) in Portainer.

It supports both CE and BE versions of Portainer. Certain features like detailed resource quotas are only fully supported in BE (licensed) versions.

---

## Example Usage

```hcl
resource "portainer_kubernetes_namespace" "test" {
  environment_id = 4
  name           = "testest"
  owner          = "terraform-test"

  annotations = {
    "owner" = "terraform"
    "env"   = "test"
  }

  resource_quota = {
    cpu         = "800m"        # CE only
    memory      = "129Mi"       # CE only
    cpu_request = "500m"        # BE only
    cpu_limit   = "1000m"       # BE only
    memory_request = "64Mi"     # BE only
    memory_limit   = "256Mi"    # BE only
  }
}
```

## ⚙️ Lifecycle & Behavior

- Terraform updates the namespace if `owner`, `annotations`, or `resource_quota` change.
- Changing the `name` triggers a destroy and recreate operation due to API limitations (Portainer does not support renaming namespaces directly in API).
- Resource quotas are applied differently depending on Portainer license:
  - **CE (Community Edition)**: only `cpu` and `memory` keys are applied.
  - **BE (Business/Enterprise Edition)**: full quota with `cpu_request`, `cpu_limit`, `memory_request`, and `memory_limit`.
- You can use `terraform destroy` to delete the namespace completely.

---

## 📥 Arguments Reference

| Name             | Type   | Required                     | Description                                                                 |
|------------------|--------|------------------------------|-----------------------------------------------------------------------------|
| `environment_id` | number | ✅ yes                       | ID of the Portainer environment (Kubernetes endpoint).                     |
| `name`           | string | ✅ yes                       | Name of the Kubernetes namespace.                                           |
| `owner`          | string | 🚫 optional (default: `""`) | Optional owner string shown in the namespace info.                          |
| `annotations`    | map    | 🚫 optional                  | Map of annotations to apply to the namespace.                               |
| `resource_quota` | object | 🚫 optional                  | CPU and memory quota. CE applies `cpu` and `memory`, BE supports `cpu_request`, `cpu_limit`, `memory_request`, `memory_limit`. |

---

### Attributes Reference

| Name  | Description                                  |
|-------|----------------------------------------------|
| `id`  | Composite ID in format `environmentID:name`  |
