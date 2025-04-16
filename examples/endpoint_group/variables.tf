variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
  # default     = "http://localhost:9000"
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
  # default     = "your-api-key-from-portainer"
}

variable "portainer_endpoint_group_name" {
  description = "Name of the Portainer endpoint group"
  type        = string
  # default     = "your-group"
}

variable "portainer_endpoint_group_description" {
  description = "Description of the group"
  type        = string
  # default     = "Description for your group"
}
