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

variable "user_id" {
  description = "ID of the user who owns the Helm repository"
  type        = number
  default     = 1
}

variable "helm_repository_url" {
  description = "URL of the Helm chart repository"
  type        = string
  default     = "https://charts.example.com"
}
