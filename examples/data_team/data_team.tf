data "portainer_team" "example" {
  name = var.team_name
}

output "team_id" {
  value = data.portainer_team.example.id
}

output "team_name" {
  value = data.portainer_team.example.name
}
