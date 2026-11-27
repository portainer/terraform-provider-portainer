variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
}

variable "client_id" {
  description = "Numeric Portainer ID of the OIDC client"
  type        = number
  default     = 1
}

variable "user_id" {
  description = "Only end this user's sessions; null ends every session of the client"
  type        = number
  default     = null
}

variable "reason" {
  description = "Change it to end the sessions again"
  type        = string
  default     = "incident-1"
}
