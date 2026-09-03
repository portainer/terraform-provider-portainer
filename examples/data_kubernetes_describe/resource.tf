data "portainer_kubernetes_describe" "web_pod" {
  environment_id = var.environment_id
  kind           = "pod"
  name           = "web-6b8f7d9c4d-abcde"
  namespace      = "prod"
}

output "web_pod_details" {
  value = data.portainer_kubernetes_describe.web_pod.describe
}
