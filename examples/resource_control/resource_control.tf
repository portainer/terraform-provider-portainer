resource "portainer_resource_control" "example" {
  resource_id         = var.resource_id
  type                = var.resource_type
  administrators_only = var.administrators_only
  public              = var.public
  sub_resource_ids    = var.sub_resource_ids
  teams               = var.teams
  users               = var.users
}
