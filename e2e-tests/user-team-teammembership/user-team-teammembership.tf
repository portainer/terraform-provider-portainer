resource "portainer_user" "your_user" {
  username = var.portainer_user_username
  password = var.portainer_user_password
  role     = var.portainer_user_role
}

resource "portainer_team" "your_team" {
  name = var.portainer_team_name
}

resource "portainer_team_membership" "your_membership" {
  role    = var.team_membership_role
  team_id = portainer_team.your_team.id
  user_id = portainer_user.your_user.id
}

data "portainer_user" "test_lookup" {
  username = portainer_user.your_user.username
}

data "portainer_team" "test_lookup" {
  name = portainer_team.your_team.name
}

output "found_user_id" {
  value = data.portainer_user.test_lookup.id
}

output "found_team_id" {
  value = data.portainer_team.test_lookup.id
}
