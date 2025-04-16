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

variable "portainer_backup_password" {
  description = "Password used to encrypt the Portainer backup"
  type        = string
  sensitive   = true
  # default     = "securepassword"
}

variable "portainer_backup_output_path" {
  description = "Path to store the output backup file"
  type        = string
  # default     = "backup.tar.gz"
}
