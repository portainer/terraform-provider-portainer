# Resource Documentation: `portainer_endpoint_trust`

# portainer_endpoint_trust

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Grants trust to an edge environment sitting in Portainer's waiting room, and optionally places it in a group, edge groups and tags as it is admitted.

The environment itself is created by the agent when it first calls home, not by Terraform. This resource adopts an existing environment rather than creating one, which is why `endpoint_id` refers to something Terraform did not make.

## Example Usage

```hcl
data "portainer_edge_waiting_room" "pending" {}

resource "portainer_endpoint_trust" "fleet" {
  for_each = toset([for e in data.portainer_edge_waiting_room.pending.environments : tostring(e.id)])

  endpoint_id    = tonumber(each.value)
  group_id       = portainer_endpoint_group.shops.id
  edge_group_ids = [portainer_edge_group.shops.id]
  tag_ids        = [portainer_tag.pos.id]
}
```

Trusting one known environment:

```hcl
resource "portainer_endpoint_trust" "shop_floor_3" {
  endpoint_id = 12
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `endpoint_id` | number | ✅ yes | Identifier of the edge environment to trust. Changing it forces a new resource. |
| `group_id` | number | ❌ no | Endpoint group to place the environment in when trust is granted. |
| `edge_group_ids` | list(number) | ❌ no | Edge groups to add the environment to when trust is granted. |
| `tag_ids` | list(number) | ❌ no | Tags to apply to the environment when trust is granted. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `name` | string | Name of the trusted environment. |
| `edge_id` | string | Edge identifier the agent reports. |
| `trusted` | bool | Whether Portainer reports the environment as trusted. |

`group_id`, `edge_group_ids` and `tag_ids` are only honoured by the call that grants trust, so changing one forces a new resource. Since destroying this resource does not call Portainer, the replacement is a single trust call - but the plan says what it will do rather than reporting an update that quietly changed nothing.

Each field is omitted from the request unless it is configured. Portainer reads an empty array as "clear this", so leaving `tag_ids` unset keeps the tags the environment already carries rather than stripping them.

Portainer has no endpoint to revoke trust. Destroying this resource stops managing it and leaves the environment trusted; removing the environment is how trust is withdrawn.

## Import

```bash
terraform import portainer_endpoint_trust.shop_floor_3 12
```

The import ID is the environment identifier. Import only records that the environment is trusted - the relation arguments are not read back, because Portainer reports the merged result and importing it would make Terraform plan a replacement against its own configuration.
