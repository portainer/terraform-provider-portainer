# Data Source Documentation: `portainer_kubernetes_describe`

# portainer_kubernetes_describe
Returns the `kubectl describe` output for a single Kubernetes resource, which is the fastest way to see why an object is unhealthy.

## Example Usage

```hcl
data "portainer_kubernetes_describe" "web_pod" {
  environment_id = 1
  kind           = "pod"
  name           = "web-6b8f7d9c4d-abcde"
  namespace      = "prod"
}

output "web_pod_details" {
  value = data.portainer_kubernetes_describe.web_pod.describe
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `environment_id` | number | ✅ yes | Environment (endpoint) identifier of the Kubernetes environment. |
| `kind` | string | ✅ yes | Kind of the resource, as `kubectl describe` takes it (`pod`, `deployment`, `node`, ...). |
| `name` | string | ✅ yes | Name of the resource. |
| `namespace` | string | ❌ no | Namespace of the resource. Leave unset for a cluster-scoped kind such as `node`. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `describe` | string | The `kubectl describe` output, as plain text. |
