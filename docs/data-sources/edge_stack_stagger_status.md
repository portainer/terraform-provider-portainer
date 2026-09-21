# Data Source Documentation: `portainer_edge_stack_stagger_status`

# portainer_edge_stack_stagger_status

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Reports how an edge stack's staggered rollout is going.

## Example Usage

```hcl
data "portainer_edge_stack_stagger_status" "monitoring" {
  edge_stack_id = portainer_edge_stack.monitoring.id
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `edge_stack_id` | number | ✅ yes | Identifier of the edge stack whose staggered rollout is reported on. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `status` | string | How the staggered rollout is going, as Portainer reports it. |

This only means anything for an edge stack deployed with a stagger configuration.
