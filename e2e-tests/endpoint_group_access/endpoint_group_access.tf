resource "portainer_team" "test" {
  name = var.team_name
}

resource "portainer_endpoint_group" "test" {
  name        = var.endpoint_group_name
  description = var.endpoint_group_description
}

resource "portainer_endpoint_group_access" "test" {
  endpoint_group_id = portainer_endpoint_group.test.id
  team_id           = portainer_team.test.id
  role_id           = 0
}