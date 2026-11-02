resource "portainer_kubernetes_environment_config" "cluster" {
  endpoint_id = var.endpoint_id

  use_load_balancer               = true
  use_server_metrics              = true
  enable_resource_over_commit     = false
  resource_over_commit_percentage = 80
  restrict_default_namespace      = true

  storage_class {
    name                   = "local-path"
    provisioner            = "rancher.io/local-path"
    access_modes           = ["RWO"]
    allow_volume_expansion = false
  }
}
