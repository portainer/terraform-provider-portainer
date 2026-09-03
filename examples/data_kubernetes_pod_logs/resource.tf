data "portainer_kubernetes_pod_logs" "web" {
  environment_id = var.environment_id
  namespace      = "default"
  pod_name       = "web-6b8f7d9c4d-abcde"
  container      = "web"
  tail_lines     = 200
  timestamps     = true

  # Read the log of the previous instance instead, to debug a crash loop.
  previous = false
}

output "web_log_tail" {
  value = data.portainer_kubernetes_pod_logs.web.logs
}

output "web_log_line_count" {
  value = data.portainer_kubernetes_pod_logs.web.line_count
}
