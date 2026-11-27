terraform {
  # Ephemeral resources need Terraform >= 1.10 (or OpenTofu >= 1.11).
  required_version = ">= 1.10"
  required_providers {
    portainer = {
      source = "portainer/portainer"
    }
    restapi = {
      source = "Mastercard/restapi"
    }
  }
}

provider "portainer" {
  endpoint = var.portainer_url
  api_key  = var.portainer_api_key
}

# Logs in when Terraform opens it, logs out (revoking the JWT) when the run ends.
ephemeral "portainer_auth" "session" {
  username = var.portainer_user
  password = var.portainer_password
}

provider "restapi" {
  uri = var.portainer_url
  headers = {
    Authorization = "Bearer ${ephemeral.portainer_auth.session.jwt}"
  }
}

data "restapi_object" "me" {
  path         = "/users"
  search_key   = "Username"
  search_value = var.portainer_user
  id_attribute = "Id"
}
