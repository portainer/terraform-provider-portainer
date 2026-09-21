# Data Source Documentation: `portainer_current_user_authorizations`

# portainer_current_user_authorizations

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Reports what the token's own user may do on an environment, both cluster-wide and per namespace.

## Example Usage

```hcl
data "portainer_current_user_authorizations" "prod" {
  endpoint_id = portainer_environment.prod.id
}

output "can_manage_prod" {
  value = contains(data.portainer_current_user_authorizations.prod.authorizations, "EndpointResourcesAccess")
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `endpoint_id` | number | ✅ yes | Identifier of the environment to report the current user's authorizations on. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `authorizations` | list(string) | Authorizations the token's user holds on the environment, sorted. |
| `namespace_authorizations` | list(object) | Per-namespace authorizations. Empty for administrators, for non-Kubernetes environments, and when the user has cluster-wide access. |

### `namespace_authorizations`

| Name | Type | Description |
|------|------|-------------|
| `authorizations` | list(string) | Authorizations the user holds in that namespace, sorted. |
| `namespace` | string | Name of the namespace. |

This folds Portainer's two endpoints - the environment-wide authorizations and the per-namespace map - into one read, because they answer the same question about the same user on the same environment.

`namespace_authorizations` is empty for administrators, for non-Kubernetes environments, and when the user has cluster-wide access; an empty list there does not mean no access.
