# Data Source Documentation: `portainer_agent_versions`

# portainer_agent_versions

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Lists the agent versions running across the environments the token can see.

## Example Usage

```hcl
data "portainer_agent_versions" "fleet" {}

output "fleet_on_one_version" {
  value = length(data.portainer_agent_versions.fleet.versions) == 1
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `versions` | list(string) | Agent versions in use across the environments the token can see, sorted. More than one entry means the fleet is not on a single version. |

This is a different question from `portainer_edge_update_schedule_info`, which reports the versions an update schedule can move environments *to*. More than one entry here means the fleet is not on a single version.
