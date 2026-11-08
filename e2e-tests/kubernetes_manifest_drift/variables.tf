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

variable "endpoint_id" {
  description = "ID of the Kubernetes environment (the e2e cluster is 4)"
  type        = number
  default     = 4
}

variable "image" {
  description = "Image of the Deployment; changing it has to update the Deployment in place"
  type        = string
  default     = "nginx:1.29-alpine"
}

variable "with_feature" {
  description = "Adds the ConfigMap key feature; dropping it again has to remove the key"
  type        = bool
  default     = false
}
