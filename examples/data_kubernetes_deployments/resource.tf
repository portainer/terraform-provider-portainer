# Cluster-wide: omit namespace to list deployments in every accessible namespace.
data "portainer_kubernetes_deployments" "all" {
  environment_id = var.environment_id
}

data "portainer_kubernetes_deployments" "default_ns" {
  environment_id = var.environment_id
  namespace      = "default"
  label_selector = "app.kubernetes.io/managed-by=terraform"
}

output "deployment_names" {
  value = [for d in data.portainer_kubernetes_deployments.all.deployments : "${d.namespace}/${d.name}"]
}

# A rollout is still in progress while observed_generation trails generation.
output "rollouts_in_progress" {
  value = [
    for d in data.portainer_kubernetes_deployments.all.deployments : "${d.namespace}/${d.name}"
    if d.observed_generation < d.generation || d.ready_replicas < d.replicas
  ]
}
