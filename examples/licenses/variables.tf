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

variable "license_key" {
  description = "Portainer license key to apply"
  type        = string
  sensitive   = true
}

variable "license_force" {
  description = "Whether to force overwrite conflicting licenses"
  type        = bool
  default     = false
}
