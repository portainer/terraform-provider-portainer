data "portainer_gitops_helm_values" "monitoring" {
  repository = "https://github.com/example/platform"
  reference  = "refs/heads/main"

  values_files = [
    "monitoring/values.yaml",
    "monitoring/values-prod.yaml",
  ]
}

output "gitops_merged_values" {
  value = data.portainer_gitops_helm_values.monitoring.merged_values
}
