resource "portainer_team" "example" {
  name = var.team_name
}

# 1. Assign team access to the environment (endpoint) first
# We assume endpoint 1 is already managed or you are managing it here.
# Note: Managing existing endpoint 1 might require 'terraform import'.
resource "portainer_environment" "example" {
  name                = var.environment_name
  environment_address = var.environment_address
  public_ip           = var.public_ip
  type                = var.environment_type
  team_access_policies = {
    (portainer_team.example.id) = 2 # 2 = Standard User
  }
}

# 2. Define the registry
resource "portainer_registry" "example" {
  name = var.dockerhub_name
  url  = var.dockerhub_url
  type = 6 # DockerHub
}

# 3. Assign the registry access to the team on that environment
resource "portainer_registry_access" "example" {
  registry_id = portainer_registry.example.id
  endpoint_id = portainer_environment.example.id
  team_id     = portainer_team.example.id
}
