# Cordon and drain a node before maintenance. Every option mirrors the
# matching kubectl drain flag; the defaults are Portainer's own.
resource "portainer_kubernetes_node_drain" "maintenance" {
  environment_id = var.environment_id
  node_name      = "worker-1"

  ignore_daemon_sets    = true
  delete_empty_dir_data = true
  timeout_seconds       = 300
  grace_period_seconds  = 30
  force                 = false
  disable_eviction      = false
}
