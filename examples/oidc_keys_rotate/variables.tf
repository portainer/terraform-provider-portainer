variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
}

variable "rotation_id" {
  description = "Change it to rotate the OIDC provider's signing key again"
  type        = string
  default     = "2026-11"
}
