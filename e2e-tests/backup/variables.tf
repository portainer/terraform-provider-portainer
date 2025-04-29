variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
  default     = "https://localhost:9443"
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
  default     = "ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8="
}

variable "portainer_backup_password" {
  description = "Password used to encrypt the Portainer backup"
  type        = string
  sensitive   = true
  default     = "secure-password-for-backup"
}

variable "portainer_backup_output_path" {
  description = "Path to store the output backup file"
  type        = string
  default     = "backup.tar.gz"
}

variable "portainer_skip_ssl_verify" {
  description = "Set to true to skip TLS certificate verification (useful for self-signed certs)"
  type        = bool
  default     = true
}
