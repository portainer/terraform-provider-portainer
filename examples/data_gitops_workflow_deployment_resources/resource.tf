data "portainer_gitops_workflow_deployment_resources" "web" {
  workflow_id   = 4
  deployment_id = 21
}

output "web_deployments" {
  value = [
    for r in data.portainer_gitops_workflow_deployment_resources.web.resources :
    "${r.namespace}/${r.name}" if r.kind == "Deployment"
  ]
}
