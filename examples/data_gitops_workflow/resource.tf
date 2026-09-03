data "portainer_gitops_workflow" "web" {
  workflow_id = 5
}

output "web_workflow_healthy" {
  value = data.portainer_gitops_workflow.web.healthy
}
