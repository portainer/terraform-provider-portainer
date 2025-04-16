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

variable "endpoint_id" {
  description = "ID of the Portainer endpoint"
  type        = number
  default     = 3
}

variable "service_name" {
  description = "Name of the Docker service to force update"
  type        = string
  default     = "my_service"
}

variable "pull_image" {
  description = "Whether to pull the latest image before updating the service"
  type        = bool
  default     = true
}
