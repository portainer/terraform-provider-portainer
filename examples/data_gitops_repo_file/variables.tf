variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
}

variable "repository_url" {
  description = "URL of the Git repository"
  type        = string
}

variable "target_file" {
  description = "Path to the file to read from the repository"
  type        = string
  default     = "docker-compose.yml"
}
