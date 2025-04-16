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

variable "portainer_registry_name" {
  description = "Název registru"
  type        = string
  # default     = "DockerHub"
}

variable "portainer_registry_type" {
  description = "Typ registru (např. 6 = Docker Hub)"
  type        = number
  # default     = 6
}

variable "portainer_registry_url" {
  description = "URL adresa registru"
  type        = string
  # default     = "docker.io"
}

variable "portainer_registry_username" {
  description = "Uživatelské jméno pro registr"
  type        = string
  # default     = "docker_user"
}

variable "portainer_registry_password" {
  description = "Heslo nebo token pro přístup do registru"
  type        = string
  sensitive   = true
  # default     = "docker_token"
}
