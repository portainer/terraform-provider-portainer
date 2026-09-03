data "portainer_endpoints_summary" "fleet" {}

output "unreachable_environments" {
  value = data.portainer_endpoints_summary.fleet.down
}

output "environments_per_group" {
  value = {
    for g in data.portainer_endpoints_summary.fleet.by_group : g.group_name => g.count
  }
}
