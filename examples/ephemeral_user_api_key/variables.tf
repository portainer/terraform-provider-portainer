variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
}

variable "portainer_user" {
  description = "User the provider logs in as and the key is issued for"
  type        = string
}

variable "portainer_password" {
  description = "Password of that user"
  type        = string
  sensitive   = true
}

variable "portainer_user_id" {
  description = "Identifier of that user"
  type        = number
}
