variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
}

variable "environment_name" {
  description = "Name of the KubeSolo environment in Portainer"
  type        = string
  default     = "kubesolo-edge"
}

variable "portainer_edge_url" {
  description = "Portainer URL the Edge Agent connects back to (baked into the edge key)"
  type        = string
}
