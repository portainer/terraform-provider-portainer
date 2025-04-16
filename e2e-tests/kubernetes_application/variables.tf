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

variable "endpoint_id" {
  description = "ID of the Portainer environment (Kubernetes cluster)"
  type        = number
  default     = 4
}

variable "namespace" {
  description = "Kubernetes namespace where the application will be deployed"
  type        = string
  default     = "default"
}

variable "manifest_file" {
  description = "Path to the Kubernetes application manifest (YAML or JSON)"
  type        = string
  default     = "application.yaml"
}
