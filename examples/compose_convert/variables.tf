variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
  default     = ""
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
  default     = ""
}
