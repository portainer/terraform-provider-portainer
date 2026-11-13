variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
}

variable "kubernetes_environment_id" {
  description = "ID of the Kubernetes environment (Edge Agent 3.0.0+, tunnel mode) to install d2k in"
  type        = number
}
