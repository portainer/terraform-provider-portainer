resource "portainer_resource_control" "example" {
  resource_id         = var.resource_id
  type                = var.resource_type
  administrators_only = var.administrators_only
  public              = var.public
  teams               = var.teams
  users               = var.users
}
