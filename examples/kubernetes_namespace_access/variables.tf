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

variable "environment_id" {
  description = "ID of the Portainer environment (Kubernetes endpoint)."
  type        = number
}

variable "namespace_name" {
  description = "Name of the Kubernetes namespace to create."
  type        = string
}

variable "users_to_add" {
  description = "List of user IDs to grant access to the namespace"
  type        = list(number)
  default     = [3, 5]
}

variable "users_to_remove" {
  description = "List of user IDs to revoke access from the namespace"
  type        = list(number)
  default     = []
}

variable "teams_to_add" {
  description = "List of team IDs to grant access to the namespace"
  type        = list(number)
  default     = [7]
}

variable "teams_to_remove" {
  description = "List of team IDs to revoke access from the namespace"
  type        = list(number)
  default     = []
}
