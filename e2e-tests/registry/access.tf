resource "portainer_team" "test_team" {
  name = var.team_name
}

# 1. Assign team access to the environment (endpoint) first
resource "portainer_environment" "test_env" {
  name                = var.environment_name
  environment_address = var.environment_address
  public_ip           = var.public_ip
  type                = var.environment_type
  team_access_policies = {
    (portainer_team.test_team.id) = 2 # Standard User
  }
}

# 2. Define the registry
#
# The name is derived rather than var.custom_name itself: custom.tf already
# creates a registry under that name, and Portainer rejects a second one with
# "Another registry with the same name already exists". This registry only
# exists to have something for the access resource below to point at.
resource "portainer_registry" "test_registry" {
  name           = "${var.custom_name} Access"
  url            = var.custom_url
  type           = 3 # Custom
  authentication = var.custom_authentication
}

# 3. Assign the registry access to the team on that environment
resource "portainer_registry_access" "test_access" {
  registry_id = portainer_registry.test_registry.id
  endpoint_id = portainer_environment.test_env.id
  team_id     = portainer_team.test_team.id
}
