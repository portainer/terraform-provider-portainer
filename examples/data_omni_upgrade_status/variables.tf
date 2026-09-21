variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
}

variable "secret" {
  description = "Credential used by this example"
  type        = string
  sensitive   = true
  default     = ""
}
