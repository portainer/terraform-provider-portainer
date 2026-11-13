# Kubernetes fields new in 3.0: prune on a Git stack and its webhook (which
# answers 202 since 3.0), namespace quotas with their effective ceiling, and
# how a user last authenticated.
resource "portainer_stack" "e2e" {
  name                      = "e2e-k8s-prune"
  deployment_type           = "kubernetes"
  method                    = "repository"
  endpoint_id               = var.k8s_environment_id
  repository_url            = "https://github.com/stefanprodan/podinfo"
  repository_reference_name = "refs/heads/master"
  file_path_in_repository   = "kustomize/deployment.yaml"
  namespace                 = "default"
  stack_webhook             = true
  kubernetes_prune          = true
}

resource "portainer_stack_webhook" "e2e" {
  webhook_id = portainer_stack.e2e.webhook_id
}

resource "portainer_kubernetes_namespace" "e2e" {
  environment_id = var.k8s_environment_id
  name           = "e2e-quota"
  resource_quota = {
    cpu_request    = "500m"
    cpu_limit      = "1000m"
    memory_request = "64Mi"
    memory_limit   = "256Mi"
  }
}

data "portainer_user" "admin" {
  username = "admin"
}

output "quota_ceiling" {
  value = portainer_kubernetes_namespace.e2e.effective_ceiling
}

output "admin_last_authentication_method" {
  value = data.portainer_user.admin.last_authentication_method
}
