variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
  # default     = "https://localhost:9443"
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
  # default     = "your-api-key-from-portainer"
}

variable "alerting_enabled" {
  description = "Whether alerting is enabled"
  type        = bool
  default     = true
}
