terraform {
  required_providers {
    portainer = {
      source = "localdomain/local/portainer"
    }
  }
}

provider "portainer" {
  endpoint = var.portainer_url
  api_key  = var.portainer_api_key
}
