data "portainer_gitops_repo_file" "compose" {
  repository_url = var.repository_url
  reference      = "refs/heads/main"
  target_file    = var.target_file
}

output "file_content" {
  value = data.portainer_gitops_repo_file.compose.file_content
}
