# Data Source Documentation: `portainer_edge_update_schedule_info`

# portainer_edge_update_schedule_info

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Reports what an edge update schedule could do: how much of the fleet is behind, and which agent versions are on offer.

## Example Usage

```hcl
data "portainer_edge_update_schedule_info" "fleet" {}

output "agents_behind" {
  value = data.portainer_edge_update_schedule_info.fleet.outdated_count
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `agent_versions` | list(string) | Agent versions an update schedule can move environments to, sorted. |
| `has_local_time_zone` | bool | Whether any environment reports a local timezone, which is what lets a schedule run in local time. |
| `has_no_local_time_zone` | bool | Whether any environment reports no local timezone. Those environments cannot be scheduled in local time. |
| `minimum_agent_version` | string | Lowest agent version that can take part in an update schedule. |
| `outdated_count` | number | Environments on an older agent. This is the number an update schedule would act on. |
| `up_to_date_count` | number | Environments already on the newest agent. |

This folds Portainer's two argument-less endpoints - the fleet state and the supported agent versions - into one read, because they answer the same question. The versions are sorted so two identical plans produce identical output.
