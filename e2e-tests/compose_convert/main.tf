terraform {
  required_providers {
    portainer = {
      source = "portainer/portainer"
    }
    local = {
      source = "hashicorp/local"
    }
  }
}

provider "portainer" {
  endpoint = var.portainer_url
  api_key  = var.portainer_api_key
}
