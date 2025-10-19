resource "portainer_kubernetes_namespace_access" "test" {
  environment_id = var.environment_id
  name           = var.namespace_name

  users_to_add    = var.users_to_add
  users_to_remove = var.users_to_remove
  teams_to_add    = var.teams_to_add
  teams_to_remove = var.teams_to_remove
}
