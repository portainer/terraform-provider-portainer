# Data Source Documentation: `portainer_endpoints_summary`

# portainer_endpoints_summary
Counts environments by health, by platform and by group — the fleet-level view of a Portainer instance.

## Example Usage

```hcl
data "portainer_endpoints_summary" "fleet" {}

output "unreachable_environments" {
  value = data.portainer_endpoints_summary.fleet.down
}

output "environments_per_group" {
  value = {
    for g in data.portainer_endpoints_summary.fleet.by_group : g.group_name => g.count
  }
}
```

## Arguments Reference

This data source takes no arguments.

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `total` | number | Total environments. |
| `up` | number | Environments Portainer can reach. |
| `down` | number | Environments Portainer cannot reach. |
| `heartbeat` | number | Edge environments whose agent is checking in. |
| `outdated` | number | Environments running an agent older than the server. |
| `unassigned` | number | Environments still in the Unassigned group. |
| `docker` | number | Docker environments. |
| `kubernetes` | number | Kubernetes environments. |
| `azure` | number | Azure ACI environments. |
| `podman` | number | Podman environments. |
| `by_group` | list | Count per environment group. Each entry has `group_id`, `group_name` and `count`. |
