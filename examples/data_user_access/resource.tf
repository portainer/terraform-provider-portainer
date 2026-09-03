# Which account is Terraform authenticating as?
data "portainer_user_access" "me" {}

# What can a specific user actually reach, and through which policy?
data "portainer_user_access" "alice" {
  user_id = 8
}

output "terraform_runs_as" {
  value = data.portainer_user_access.me.username
}

output "alice_environments" {
  value = [
    for a in data.portainer_user_access.alice.effective_access :
    "${a.endpoint_name} - ${a.role_name} (via ${a.access_location})"
  ]
}
