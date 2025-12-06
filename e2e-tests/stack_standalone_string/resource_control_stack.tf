resource "portainer_resource_control" "standalone_string_rc" {
  resource_id         = portainer_stack.standalone_string.id
  type                = 6
  administrators_only = false
  public              = true
  teams               = []
  users               = []
}
