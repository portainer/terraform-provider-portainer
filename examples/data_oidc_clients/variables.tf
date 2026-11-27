variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
}

variable "client_name" {
  description = "Name of the OIDC client to look up"
  type        = string
  default     = "Grafana"
}
