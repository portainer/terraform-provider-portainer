data "portainer_kubernetes_service_account" "default" {
  endpoint_id = 1
  namespace   = "default"
  name        = "default"
}

output "kubernetes_service_account_automounts" {
  value = data.portainer_kubernetes_service_account.default.automount_service_account_token
}
