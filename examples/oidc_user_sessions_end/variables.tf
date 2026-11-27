variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
}

variable "user_id" {
  description = "Portainer user to sign out of every OIDC client"
  type        = number
  default     = 2
}

variable "reason" {
  description = "Change it to sign the user out again"
  type        = string
  default     = "offboarding"
}
