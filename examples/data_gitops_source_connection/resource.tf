# Validate the credentials before a source is created.
data "portainer_gitops_source_connection" "check" {
  url           = "https://github.com/acme/infra.git"
  username      = "ci-bot"
  password      = var.git_token
  fail_on_error = true
}

# Re-test a source that already exists.
data "portainer_gitops_source_connection" "stored" {
  source_id = 12
}

output "stored_source_reachable" {
  value = data.portainer_gitops_source_connection.stored.success
}
