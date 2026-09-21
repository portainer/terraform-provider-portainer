resource "portainer_edge_stack" "string_example" {
  name                    = var.edge_stack_name
  stack_file_content      = var.edge_stack_file_content
  deployment_type         = var.edge_stack_deployment_type
  edge_groups             = var.edge_stack_edge_groups
  registries              = var.edge_stack_registries
  use_manifest_namespaces = var.edge_stack_use_manifest_namespaces
}

# Business Edition only: deploy the stack from a Helm chart repository.
resource "portainer_edge_stack" "monitoring" {
  name            = "monitoring"
  deployment_type = 1 # Helm charts deploy to Kubernetes
  edge_groups     = [1]

  helm_config {
    chart_url     = "https://prometheus-community.github.io/helm-charts"
    chart_name    = "kube-prometheus-stack"
    chart_version = "51.2.0"
    namespace     = "monitoring"
    atomic        = true
    timeout       = "5m0s"

    values_inline = yamlencode({
      grafana = {
        enabled = true
      }
    })
  }
}
