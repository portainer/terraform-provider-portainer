data "portainer_gitops_repo_refs" "example" {
  repository_url = var.repository_url
}

output "git_refs" {
  value = data.portainer_gitops_repo_refs.example.refs
}
