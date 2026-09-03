# Resource Documentation: `portainer_endpoint_relations`

# portainer_endpoint_relations
The `portainer_endpoint_relations` resource applies edge groups, tags and the environment group to several environments in one call. Portainer applies the whole payload in a single transaction, so either all of the relations land or none do.

Use it when relations are decided in one place for a fleet. For a single environment, `portainer_environment` (`group_id`, `tag_ids`) or `portainer_endpoint_group_membership` are the more direct tools; using both for the same environment makes them fight over it on every apply.

## Example Usage

```hcl
resource "portainer_endpoint_relations" "fleet" {
  relation {
    endpoint_id    = portainer_environment.branch_a.id
    edge_group_ids = [portainer_edge_group.stores.id]
    tag_ids        = [portainer_tag.retail.id]
    group_id       = portainer_endpoint_group.stores.id
  }

  relation {
    endpoint_id    = portainer_environment.branch_b.id
    edge_group_ids = [portainer_edge_group.stores.id]
  }
}
```

## Arguments Reference

| Name       | Type | Required | Description                                                     |
|------------|------|----------|-----------------------------------------------------------------|
| `relation` | list | ✅ yes   | One block per environment. At least one is required.             |

Each `relation` block accepts:

| Name             | Type   | Required | Default | Description                                                                       |
|------------------|--------|----------|---------|-----------------------------------------------------------------------------------|
| `endpoint_id`    | number | ✅ yes   | –       | Identifier of the environment.                                                     |
| `edge_group_ids` | list   | ❌ no    | –       | Edge groups the environment belongs to. An empty list leaves them untouched.        |
| `tag_ids`        | list   | ❌ no    | –       | Tags assigned to the environment. An empty list leaves them untouched.              |
| `group_id`       | number | ❌ no    | `0`     | Environment group to move the environment to. Zero leaves its group untouched.      |

## Attributes Reference

| Name | Description                              |
|------|------------------------------------------|
| `id` | Synthetic identifier for the apply.       |

Portainer offers no endpoint to read relations back as a set or to clear them, so this resource cannot detect drift, and destroying it only stops managing the relations — the environments keep what they were given.
