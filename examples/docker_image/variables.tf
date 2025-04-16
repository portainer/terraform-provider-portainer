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
  description = "ID of the Portainer environment where the image should be pulled"
  type        = number
}

variable "image" {
  description = "Docker image including tag (e.g., nginx:alpine)"
  type        = string
}
