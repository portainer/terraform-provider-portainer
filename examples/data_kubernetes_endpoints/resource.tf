data "portainer_kubernetes_endpoints" "all" {
  endpoint_id = 1
}

output "kubernetes_services_without_backends" {
  value = [
    for e in data.portainer_kubernetes_endpoints.all.endpoints : "${e.namespace}/${e.name}"
    if length(e.addresses) == 0
  ]
}
