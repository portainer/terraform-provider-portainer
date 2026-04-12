data "portainer_helm_git_dryrun" "test" {
  endpoint_id    = var.endpoint_id
  repository_url = var.repository_url
  reference_name = "refs/heads/main"
  chart_path     = "charts/my-app"
  release_name   = "my-app"
  namespace      = "default"
  values_files   = ["values.yaml"]
}

output "rendered_manifest" {
  value = data.portainer_helm_git_dryrun.test.manifest
}
