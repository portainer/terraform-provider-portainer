# Read-only Kubernetes data sources against the k3d cluster.
#
# The metrics data sources (pod/node metrics, application resources) are
# deliberately absent: they need metrics-server to have finished starting, which
# makes them a timing race in CI rather than a test of the provider.

data "portainer_kubernetes_cluster" "prod" {
  environment_id = var.endpoint_id
}

data "portainer_kubernetes_nodes" "prod" {
  environment_id = var.endpoint_id
}

data "portainer_kubernetes_events" "prod" {
  environment_id = var.endpoint_id
}

data "portainer_kubernetes_persistent_volumes" "prod" {
  environment_id = var.endpoint_id
}

data "portainer_kubernetes_config" "prod" {
  environment_ids = [var.endpoint_id]
}

# Describes the first node the cluster reports, so the name does not have to be
# hard-coded for the CI cluster.
data "portainer_kubernetes_describe" "first_node" {
  environment_id = var.endpoint_id
  kind           = "node"
  name           = data.portainer_kubernetes_nodes.prod.nodes[0].name
}

output "cluster_version" {
  value = data.portainer_kubernetes_cluster.prod.version

  precondition {
    condition     = data.portainer_kubernetes_cluster.prod.version != ""
    error_message = "the cluster must report a Kubernetes version."
  }
}

output "node_count" {
  value = length(data.portainer_kubernetes_nodes.prod.nodes)

  precondition {
    condition     = length(data.portainer_kubernetes_nodes.prod.nodes) > 0
    error_message = "a k3d cluster always has at least one node."
  }
}

output "node_ready" {
  value = data.portainer_kubernetes_nodes.prod.nodes[0].ready
}

output "kubeconfig_generated" {
  # The kubeconfig is sensitive; whether one was generated is not. The mark is
  # dropped explicitly so the check stays readable in the run output instead
  # of being hidden behind sensitive = true.
  value = nonsensitive(data.portainer_kubernetes_config.prod.kubeconfig != "")

  precondition {
    condition     = data.portainer_kubernetes_config.prod.kubeconfig != ""
    error_message = "the generated kubeconfig must not be empty."
  }
}

output "describe_output" {
  value = length(data.portainer_kubernetes_describe.first_node.describe) > 0
}

output "namespace_count" {
  value = data.portainer_kubernetes_cluster.prod.namespaces_count
}
