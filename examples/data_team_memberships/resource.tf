data "portainer_team_memberships" "platform" {
  team_id = 1
}

output "platform_leaders" {
  value = data.portainer_team_memberships.platform.leader_user_ids
}
