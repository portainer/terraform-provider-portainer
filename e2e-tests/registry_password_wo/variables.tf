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

variable "portainer_skip_ssl_verify" {
  description = "Set to true to skip TLS certificate verification (useful for self-signed certs)"
  type        = bool
  default     = true
}

variable "registry_password" {
  description = "Password sent through password_wo. run.sh passes a new one with a higher password_wo_version to rotate it."
  type        = string
  sensitive   = true
  default     = "e2e-first-secret"
}

variable "registry_password_version" {
  description = "Version of password_wo."
  type        = number
  default     = 1
}
