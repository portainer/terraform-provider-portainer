# Full cycle: the key is created on apply and revoked on destroy, which is the
# gap this resource exists to close - portainer_user could only ever create one.
#
# The key is created for the account the provider is signed in as. Portainer
# lets a user create a key only for themselves, and a provider authenticates
# when Terraform configures it - before any resource exists - so a user created
# by this same configuration could never be signed in as.

data "portainer_user" "self" {
  username = var.portainer_username
}

resource "portainer_user_api_key" "test" {
  user_id     = tonumber(data.portainer_user.self.id)
  description = "e2e"
  password    = var.portainer_password
}

output "api_key_prefix" {
  value = portainer_user_api_key.test.prefix

  # Portainer returns the raw key exactly once, at creation.
  precondition {
    condition     = portainer_user_api_key.test.raw_api_key != ""
    error_message = "Portainer must return the raw API key on creation."
  }
}
