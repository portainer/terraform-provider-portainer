resource "portainer_docker_secret" "example_secret" {
  endpoint_id = var.endpoint_id
  name        = var.secret_name
  data        = base64encode(var.secret_data)

  labels     = var.secret_labels
  templating = var.secret_templating
}

resource "portainer_team" "your_example_team" {
  name = var.portainer_team_name
}

resource "portainer_resource_control" "secret_access" {
  resource_control_id = portainer_docker_secret.example_secret.resource_control_id
  type                = var.resource_control_type
  administrators_only = var.resource_control_administrators_only
  public              = var.resource_control_public
  teams               = [portainer_team.your_example_team.id]
}
