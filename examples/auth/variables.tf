variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
  # default     = "http://localhost:9000"
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  default     = "some-fake-api-token"
}

variable "portainer_username" {
  type        = string
  description = "Portainer username"
  sensitive   = true
  # default = "admin"
}

variable "portainer_password" {
  type        = string
  description = "Portainer password"
  sensitive   = true
  # default = "password"
}