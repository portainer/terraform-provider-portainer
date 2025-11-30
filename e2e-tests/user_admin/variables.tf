variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
  default     = "https://localhost:9443"
}

variable "portainer_skip_ssl_verify" {
  description = "Set to true to skip TLS certificate verification (useful for self-signed certs)"
  type        = bool
  default     = true
}

variable "portainer_username" {
  type        = string
  description = "Portainer username"
  sensitive   = true
  default     = "admin"
}

variable "portainer_password" {
  type        = string
  description = "Portainer password"
  sensitive   = true
  default     = "password123456789"
}