terraform {
  required_providers {
    portainer = {
      source = "portainer/portainer"
    }
  }
}

# The admin session creates the user.
provider "portainer" {
  endpoint        = var.portainer_url
  api_key         = var.portainer_api_key
  skip_ssl_verify = var.portainer_skip_ssl_verify
}

# Portainer accepts POST /users/{id}/tokens only from a session, and only for
# the calling user's own account - so the key is created by a second provider
# authenticated as that user rather than by the admin.
provider "portainer" {
  alias           = "as_key_owner"
  endpoint        = var.portainer_url
  api_user        = var.user_username
  api_password    = var.user_password
  skip_ssl_verify = var.portainer_skip_ssl_verify
}
