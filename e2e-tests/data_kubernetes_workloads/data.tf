# Read-only coverage of the Kubernetes list endpoints added in Portainer 2.45.
# Nothing here mutates the cluster, so the whole file is apply-only.

data "portainer_kubernetes_ingress_classes" "all" {
  environment_id = var.endpoint_id
}

data "portainer_kubernetes_resource_quotas" "ns" {
  environment_id = var.endpoint_id
  namespace      = var.namespace
}

data "portainer_kubernetes_pods" "cluster" {
  environment_id = var.endpoint_id
}

data "portainer_kubernetes_deployments" "cluster" {
  environment_id = var.endpoint_id
}

data "portainer_kubernetes_replicasets" "ns" {
  environment_id = var.endpoint_id
  namespace      = var.namespace
}

# A k3d cluster always runs system pods, so an empty list means the cluster-wide
# route did not work rather than an idle cluster.
output "pod_count" {
  value = length(data.portainer_kubernetes_pods.cluster.pods)

  precondition {
    condition     = length(data.portainer_kubernetes_pods.cluster.pods) > 0
    error_message = "The cluster-wide pod listing must return at least the system pods."
  }
}

output "deployment_count" {
  value = length(data.portainer_kubernetes_deployments.cluster.deployments)
}

output "ingress_class_names" {
  value = [for c in data.portainer_kubernetes_ingress_classes.all.ingress_classes : c.name]
}

output "resource_quota_names" {
  value = [for q in data.portainer_kubernetes_resource_quotas.ns.resource_quotas : q.name]
}

output "replicaset_revisions" {
  value = [for rs in data.portainer_kubernetes_replicasets.ns.replicasets : rs.revision]
}
