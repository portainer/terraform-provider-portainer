data "portainer_gitops_source" "infra" {
  source_id = 12
}

# A source cannot be deleted while a workflow still uses it.
output "infra_workflows" {
  value = [for w in data.portainer_gitops_source.infra.workflows : w.name]
}
