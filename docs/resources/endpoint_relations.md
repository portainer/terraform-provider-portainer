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
| `edge_group_ids` | list   | ❌ no    | –       | Edge groups the environment belongs to, replacing whatever it had. Omit to leave them alone. |
| `tag_ids`        | list   | ❌ no    | –       | Tags assigned to the environment, replacing whatever it had. Omit to leave them alone.       |
| `group_id`       | number | ❌ no    | `0`     | Environment group to move the environment to. Zero leaves its group untouched.      |

## Attributes Reference

| Name | Description                              |
|------|------------------------------------------|
| `id` | Synthetic identifier for the apply.       |

Each field is sent only when it is configured, because that is how Portainer reads the payload: it acts on tags and edge groups only when they are present, and on the group only when non-zero. An **empty list is not a no-op on the Portainer side — it clears** that environment's tags or edge groups, so this resource omits the field instead.

The consequence is that clearing tags or edge groups cannot be expressed here: Portainer cannot tell an intentional "none" from "do not touch". Use `tag_ids` on `portainer_environment` when you need to empty them.

Portainer offers no endpoint to read relations back as a set, so this resource cannot detect drift, and destroying it only stops managing the relations — the environments keep what they were given.
