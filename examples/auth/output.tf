output "jwt_token" {
  value     = portainer_auth.login.jwt
  sensitive = true
}