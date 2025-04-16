resource "portainer_docker_network" "test_bridge" {
  endpoint_id = var.endpoint_id
  name        = var.network_name
  driver      = var.network_driver
  internal    = var.network_internal
  attachable  = var.network_attachable
  ingress     = var.network_ingress
  config_only = var.network_config_only
  config_from = var.network_config_from
  enable_ipv4 = var.network_enable_ipv4
  enable_ipv6 = var.network_enable_ipv6

  options = var.network_options
  labels  = var.network_labels
}
