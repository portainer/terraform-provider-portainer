data "portainer_gitops_sources" "all" {
  sort  = "name"
  order = "asc"
}

output "broken_sources" {
  value = [
    for s in data.portainer_gitops_sources.all.sources : "${s.name}: ${s.status_error}"
    if s.status == "error"
  ]
}

output "source_status_counts" {
  value = data.portainer_gitops_sources.all.summary[0]
}
