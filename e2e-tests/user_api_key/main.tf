terraform {
  required_providers {
    portainer = {
      source = "portainer/portainer"
    }
  }
}

# Portainer accepts POST /users/{id}/tokens only from a session, so this
# directory authenticates with a username and password rather than the API key
# the rest of the suite uses.
provider "portainer" {
  endpoint        = var.portainer_url
  api_user        = var.portainer_username
  api_password    = var.portainer_password
  skip_ssl_verify = var.portainer_skip_ssl_verify
}
