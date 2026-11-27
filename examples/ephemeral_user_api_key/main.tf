terraform {
  # Ephemeral resources need Terraform >= 1.10 (or OpenTofu >= 1.11).
  required_version = ">= 1.10"
  required_providers {
    portainer = {
      source = "portainer/portainer"
    }
  }
}

# Portainer issues API keys only to a session, so the provider logs in with
# a user and password, and the key is issued for that same user.
provider "portainer" {
  endpoint     = var.portainer_url
  api_user     = var.portainer_user
  api_password = var.portainer_password
}

# Created when Terraform opens it, revoked when the run ends; never in state.
ephemeral "portainer_user_api_key" "run" {
  user_id  = var.portainer_user_id
  password = var.portainer_password
}

# A second provider configuration that authenticates with the short-lived key.
provider "portainer" {
  alias    = "with_key"
  endpoint = var.portainer_url
  api_key  = ephemeral.portainer_user_api_key.run.raw_api_key
}

data "portainer_user" "me" {
  provider = portainer.with_key
  username = var.portainer_user
}
