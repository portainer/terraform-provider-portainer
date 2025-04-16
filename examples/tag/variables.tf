variable "portainer_url" {
  description = "Default Portainer URL"
  # default     = "http://localhost:9000"
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  # default     = "your-api-key-from-portainer"
}

variable "portainer_tag_name" {
  description = "Portainer tag name"
  type        = string
  # default     = "your-default-portainer-tag"
}
