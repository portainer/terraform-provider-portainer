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
