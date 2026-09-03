data "portainer_kubernetes_nodes" "prod" {
  environment_id = var.environment_id
}

output "cordoned_nodes" {
  value = [for n in data.portainer_kubernetes_nodes.prod.nodes : n.name if n.unschedulable]
}

output "unready_nodes" {
  value = [for n in data.portainer_kubernetes_nodes.prod.nodes : n.name if !n.ready]
}
