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

variable "webhook_token" {
  description = "Webhook token to trigger service restart"
  type        = string
}

variable "stack_id" {
  description = "Stack ID to trigger git update"
  type        = string
}
