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

variable "source_url" {
  description = "Public Git repository the Source points at. Given with .git on purpose: Portainer stores it without, which used to show as a change on every plan."
  type        = string
  default     = "https://github.com/portainer/terraform-provider-portainer.git"
}

variable "template_description" {
  description = "Description of the template. run.sh changes it to exercise an update."
  type        = string
  default     = "Custom template created from a Portainer Source"
}
