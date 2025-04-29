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

variable "endpoint_id" {
  description = "ID of the Portainer environment (Kubernetes cluster)"
  type        = number
  default     = 4
}

variable "type" {
  description = "Type of Kubernetes volume. One of: persistent-volume-claim, persistent-volume, volume-attachment"
  type        = string
  default     = "persistent-volume-claim"
}


variable "namespace" {
  description = "Kubernetes namespace where the volume will be deployed"
  type        = string
  default     = "default"
}

variable "manifest_file" {
  description = "Path to the Kubernetes volume manifest (YAML or JSON)"
  type        = string
  default     = "volume.yaml"
}

variable "portainer_skip_ssl_verify" {
  description = "Set to true to skip TLS certificate verification (useful for self-signed certs)"
  type        = bool
  default     = true
}
