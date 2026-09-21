# Full cycle: the key is created on apply and revoked on destroy, which is the
# gap this resource exists to close - portainer_user could only ever create one.

resource "portainer_user" "key_owner" {
  username = "e2e-api-key-user"
  password = var.user_password
  role     = 2
}

resource "portainer_user_api_key" "test" {
  user_id     = portainer_user.key_owner.id
  description = "e2e"
  password    = var.user_password
}

output "api_key_prefix" {
  value = portainer_user_api_key.test.prefix

  # Portainer returns the raw key exactly once, at creation.
  precondition {
    condition     = portainer_user_api_key.test.raw_api_key != ""
    error_message = "Portainer must return the raw API key on creation."
  }
}
