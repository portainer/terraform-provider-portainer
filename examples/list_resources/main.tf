terraform {
  # terraform query and list resources need Terraform 1.14 or later.
  required_version = ">= 1.14.0"
  required_providers {
    portainer = {
      source  = "portainer/portainer"
      version = ">= 3.1.0"
    }
  }
}

provider "portainer" {
  endpoint = var.portainer_url
  api_key  = var.portainer_api_key
}

# Import a tag found by `terraform query` by its identity (Terraform 1.12+).
# Uncomment and set the ID from the query output.
# import {
#   to       = portainer_tag.production
#   identity = {
#     id = "1"
#   }
# }
#
# resource "portainer_tag" "production" {
#   name = "production"
# }
