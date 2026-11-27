variable "portainer_url" {
  description = "Default Portainer URL"
  # default     = "http://localhost:9000"
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  # default     = "your-api-key-from-portainer"
}

variable "environment_group_id" {
  description = "Environment group whose environments are listed"
  type        = number
  default     = 1
}

variable "docker_endpoint_id" {
  description = "Docker (or Docker Swarm) environment whose Docker objects are listed"
  type        = number
  default     = 1
}

variable "kubernetes_endpoint_id" {
  description = "Kubernetes environment whose Kubernetes objects are listed"
  type        = number
  default     = 2
}
