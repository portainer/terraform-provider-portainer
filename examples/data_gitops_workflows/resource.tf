data "portainer_gitops_workflows" "kubernetes" {
  platform = "kubernetes"
}

output "failing_workflows" {
  value = [
    for w in data.portainer_gitops_workflows.kubernetes.workflows :
    "${w.name}: source=${w.source_status} artifact=${w.artifact_status} target=${w.target_status}"
    if !w.healthy
  ]
}
