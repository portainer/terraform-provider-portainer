variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
  default     = "http://localhost:9000"
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
  default     = "ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8="
}

variable "ssl_cert_path" {
  description = "Path to the SSL certificate file"
  type        = string
  default     = "certs/server.crt"
}

variable "ssl_key_path" {
  description = "Path to the SSL private key file"
  type        = string
  default     = "certs/server.key"
}

variable "ssl_http_enabled" {
  description = "Whether to enable HTTP access in addition to HTTPS"
  type        = bool
  default     = false
}