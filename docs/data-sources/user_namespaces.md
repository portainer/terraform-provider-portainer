# Data Source Documentation: `portainer_user_namespaces`

# portainer_user_namespaces

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Reports which Kubernetes namespaces a user can reach, and what they may do in each, across every environment.

## Example Usage

```hcl
data "portainer_user_namespaces" "alice" {
  user_id = portainer_user.alice.id
}

output "alice_writable_namespaces" {
  value = [
    for n in data.portainer_user_namespaces.alice.namespaces : "${n.endpoint_id}/${n.namespace}"
    if contains(n.authorizations, "K8sAccessNamespaceWrite")
  ]
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `user_id` | number | ✅ yes | Identifier of the user whose namespace authorizations are read. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `namespaces` | list(object) | The user's namespace authorizations, one entry per environment and namespace. |
| `raw` | string | The whole response as Portainer returns it, encoded as JSON. Portainer adds authorization names between releases, so this is what to read when a name is not in the flattened list yet. |

### `namespaces`

| Name | Type | Description |
|------|------|-------------|
| `authorizations` | list(string) | Authorizations the user holds in that namespace, sorted. An empty list means the namespace is visible but nothing is permitted in it. |
| `endpoint_id` | string | Identifier of the environment the namespace belongs to. |
| `namespace` | string | Name of the namespace. |

Portainer reports authorizations as an open-ended map of name to a boolean and adds names between releases. `authorizations` lists only the names that are switched on, sorted so two identical plans produce identical output; `raw` carries the whole response for anything the flattened list does not cover yet.
