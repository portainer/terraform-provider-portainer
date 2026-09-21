data "portainer_gitops_repo_file_search" "stacks" {
  repository = "https://github.com/example/platform"
  reference  = "refs/heads/main"
  include    = "yml,yaml"
}

output "gitops_deployable_files" {
  value = data.portainer_gitops_repo_file_search.stacks.paths
}
