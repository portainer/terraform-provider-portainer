resource "portainer_gitops_source" "infra" {
  url      = "https://github.com/acme/infra.git"
  name     = "infra"
  interval = "5m"

  username = "ci-bot"
  password = var.git_token

  tls_skip_verify     = false
  administrators_only = false

  public        = false
  user_accesses = [3]
  team_accesses = [1]
}

output "infra_source_status" {
  value = portainer_gitops_source.infra.status
}
