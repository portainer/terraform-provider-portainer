resource "portainer_environment" "your-host" {
  name                = var.portainer_environment_name
  environment_address = var.portainer_environment_address
  type                = var.portainer_environment_type
}
