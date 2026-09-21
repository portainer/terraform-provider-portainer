variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
  default     = "https://localhost:9443"
}

variable "portainer_username" {
  description = "Portainer username the key is created for. Portainer only lets a user create a key for themselves."
  type        = string
  sensitive   = true
  default     = "admin"
}

variable "portainer_password" {
  description = "Password of that user"
  type        = string
  sensitive   = true
  default     = "password123456789"
}

variable "portainer_skip_ssl_verify" {
  description = "Set to true to skip TLS certificate verification (useful for self-signed certs)"
  type        = bool
  default     = true
}
