variable "portainer_url" {
  description = "Default Portainer URL"
  # default     = "http://localhost:9000"
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  # default     = "your-api-key-from-portainer"
}

variable "edge_configuration_name" {
  description = "Name of the existing edge configuration to look up"
  type        = string
}
