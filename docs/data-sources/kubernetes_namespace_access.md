# Data Source Documentation: `portainer_kubernetes_namespace_access`

# portainer_kubernetes_namespace_access

> **Requires Portainer Business Edition 3.0 or later.** On Community Edition or an older server the read fails with an error that says so.

Reads which users and teams currently have access to a Kubernetes namespace, and with which role. It is the read side of the `portainer_kubernetes_namespace_access` resource, which grants and revokes that access, and takes the same `endpoint_id` and `namespace_id` arguments.

## Example Usage

```hcl
data "portainer_kubernetes_namespace_access" "team_a" {
  endpoint_id  = 4
  namespace_id = "team-a"
}

output "users_with_access" {
  value = [for u in data.portainer_kubernetes_namespace_access.team_a.user_access : u.user_id]
}
```

## Arguments Reference

| Name           | Type   | Required | Description |
|----------------|--------|----------|-------------|
| `endpoint_id`  | number | Yes      | ID of the Kubernetes environment owning the namespace. |
| `namespace_id` | string | Yes      | Name of the namespace. |

## Attributes Reference

| Name          | Type | Description |
|---------------|------|-------------|
| `id`          | string | `<endpoint_id>:<namespace_id>`. |
| `user_access` | list | Users with access, ordered by ID: `user_id`, `role_id`, `namespaces` (for a namespace-scoped role). |
| `team_access` | list | Teams with access, ordered by ID: `team_id`, `role_id`, `namespaces`. |

Portainer answers 503 while namespace access is not yet available for the environment (for example right after it was added); the read then fails and can be retried.
