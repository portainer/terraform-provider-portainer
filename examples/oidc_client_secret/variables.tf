variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
}

variable "secret_generation" {
  description = "Increase to regenerate the client secret; the replaced one keeps working for a day"
  type        = number
  default     = 1
}
