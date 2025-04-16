resource "portainer_user" "test_user" {
  username  = var.user_username
  password  = var.user_password
  role      = var.user_role
  ldap_user = var.user_ldap
}

resource "portainer_team" "test_team" {
  name = var.team_name
}

resource "portainer_team_membership" "test_membership" {
  role    = var.team_membership_role
  team_id = portainer_team.test_team.id
  user_id = portainer_user.test_user.id
}
