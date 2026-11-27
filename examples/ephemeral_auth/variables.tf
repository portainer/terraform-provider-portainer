variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
}

variable "portainer_user" {
  description = "User to log in as"
  type        = string
}

variable "portainer_password" {
  description = "Password of that user"
  type        = string
  sensitive   = true
}
