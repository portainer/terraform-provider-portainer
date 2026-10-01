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

variable "stack_name" {
  description = "Name of the stack"
  type        = string
  default     = "nginx-error-recovery-147"
}

variable "stack_endpoint_id" {
  description = "Portainer environment/endpoint ID"
  type        = number
  default     = 3
}

variable "stack_image" {
  description = "Image the stack deploys. run.sh swaps this for an unpullable tag to drive the stack into Error, then back to a working one."
  type        = string
  default     = "nginx:1.27-alpine"
}
