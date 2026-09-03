variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
}

variable "user_password" {
  description = "Password of the user the API key is generated for"
  type        = string
  sensitive   = true
}
