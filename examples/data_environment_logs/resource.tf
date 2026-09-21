data "portainer_environment_logs" "errors" {
  endpoint_id = 1
  from        = "2026-01-01T00:00:00Z"
  to          = "2026-01-02T00:00:00Z"
  namespace   = "default"
  severity    = "error"
  limit       = 100
}

output "environment_error_logs" {
  value = [for l in data.portainer_environment_logs.errors.logs : l.message]
}
