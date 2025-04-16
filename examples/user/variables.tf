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

variable "portainer_user_username" {
  description = "Portainer username used for resource provisioning"
  type        = string
  # default     = "your-user"
}

variable "portainer_user_password" {
  description = "Portainer password used for resource provisioning"
  type        = string
  sensitive   = true
  # default     = "password"
}

variable "portainer_user_role" {
  description = "Role to assign to the Portainer user"
  type        = number
  # default     = 2 # 1 = admin, 2 = standard user
}
