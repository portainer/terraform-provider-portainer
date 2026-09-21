data "portainer_environment_metrics" "cpu" {
  endpoint_id = 1
  metric      = "cpu"
  aggregation = "avg"
  group_by    = "namespace"
  from        = "2026-01-01T00:00:00Z"
  to          = "2026-01-02T00:00:00Z"
}

output "environment_cpu_series" {
  value = jsondecode(data.portainer_environment_metrics.cpu.data)
}
