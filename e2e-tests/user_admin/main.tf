terraform {
  required_providers {
    portainer = {
      source = "portainer/portainer"
    }
  }
}

provider "portainer" {
  endpoint        = var.portainer_url
  skip_ssl_verify = var.portainer_skip_ssl_verify
}
