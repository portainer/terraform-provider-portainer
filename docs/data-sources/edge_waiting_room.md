# Data Source Documentation: `portainer_edge_waiting_room`

# portainer_edge_waiting_room

> **Business Edition.** The listing itself uses the standard `/endpoints` endpoint, but the
> waiting room and `portainer_endpoint_trust` are Business Edition features.


Lists the edge environments that have called home but have not been trusted yet - what the Portainer UI calls the waiting room.

Edge environments are created by the agents, so their identifiers are not known to a configuration until they are looked up. This data source is the companion to `portainer_endpoint_trust`.

## Example Usage

```hcl
data "portainer_edge_waiting_room" "pending" {}

output "awaiting_trust" {
  value = data.portainer_edge_waiting_room.pending.endpoint_ids
}
```

Only the environments in one group:

```hcl
data "portainer_edge_waiting_room" "shops" {
  group_id = portainer_endpoint_group.shops.id
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `group_id` | number | ❌ no | Only list environments in this endpoint group. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `endpoint_ids` | list(number) | Identifiers of the environments awaiting trust, which is what `portainer_endpoint_trust` takes. |
| `environments` | list(object) | The environments awaiting trust. |

### `environments`

| Name | Type | Description |
|------|------|-------------|
| `id` | number | Identifier of the environment. |
| `name` | string | Name of the environment. |
| `type` | number | Type of the environment: 4 = Edge Agent, 7 = Kubernetes Edge Agent. |
| `edge_id` | string | Edge identifier the agent reports. |
| `group_id` | number | Endpoint group the environment currently sits in. |
| `last_check_in` | number | Unix timestamp of the agent's most recent check-in. |

Trusting everything the data source returns will also trust anything that turns up between the plan and the apply. On a fleet where that matters, filter on `last_check_in` or pin the identifiers explicitly rather than iterating the whole list.
