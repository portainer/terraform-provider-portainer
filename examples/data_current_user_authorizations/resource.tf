data "portainer_current_user_authorizations" "prod" {
  endpoint_id = 1
}

output "current_user_authorizations" {
  value = data.portainer_current_user_authorizations.prod.authorizations
}
