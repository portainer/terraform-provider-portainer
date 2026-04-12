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

variable "stack_id" {
  description = "ID of the stack to migrate"
  type        = number
  default     = 1
}

variable "target_endpoint_id" {
  description = "ID of the target environment to migrate the stack to"
  type        = number
  default     = 2
}
