# Resource Documentation: `portainer_endpoint_group_membership`

# portainer_endpoint_group_membership
The `portainer_endpoint_group_membership` resource places a single environment in an environment group.

Use it when the environment and the group are managed separately — for example when the group is created by Terraform but the environments join it later, or when an environment is created outside Terraform. To set a group at environment creation time, use `group_id` on `portainer_environment` instead; using both for the same environment makes them fight over it on every apply.

## Example Usage

```hcl
resource "portainer_endpoint_group" "production" {
  name = "production"
}

resource "portainer_endpoint_group_membership" "prod_docker" {
  endpoint_group_id = portainer_endpoint_group.production.id
  endpoint_id       = portainer_environment.docker.id
}
```

## Arguments Reference

| Name                | Type   | Required | Description                                                                            |
|---------------------|--------|----------|----------------------------------------------------------------------------------------|
| `endpoint_group_id` | number | ✅ yes   | Identifier of the environment group. Changing it replaces the resource.                 |
| `endpoint_id`       | number | ✅ yes   | Identifier of the environment to place in the group. Changing it replaces the resource. |

## Attributes Reference

| Name | Description                                                              |
|------|--------------------------------------------------------------------------|
| `id` | Composite identifier in the form `<endpoint_group_id>/<endpoint_id>`.     |

Membership is a property of the environment (`endpoint.GroupId`), not an object of its own, so an environment can only be in one group. If something else moves it to another group, this resource disappears from state on the next refresh and the next apply moves it back.

Destroying the resource returns the environment to Portainer's Unassigned group (ID 1); there is no state in which an environment belongs to no group at all.
