variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
  default     = "http://localhost:9000"
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
  # default     = "your-api-key-from-portainer"
}

variable "webhook_id" {
  description = "Webhook token used to trigger the stack update."
  type        = string
  default     = "65001023-9dd7-415f-9cff-358ba0a78463"
}
