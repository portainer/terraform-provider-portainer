data "portainer_team_membership" "example" {
  team_id = var.team_membership_team_id
  user_id = var.team_membership_user_id
}

output "team_membership_id" {
  value = data.portainer_team_membership.example.id
}

output "team_membership_role" {
  value = data.portainer_team_membership.example.role
}
