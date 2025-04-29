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

variable "environment_id" {
  description = "The ID of the Kubernetes environment (endpoint) in Portainer where the Helm chart will be deployed"
  type        = number
  default     = 4
}

variable "helm_chart" {
  description = "The name of the Helm chart, e.g., nginx or redis"
  type        = string
  default     = "nginx"
}

variable "helm_release_name" {
  description = "The name of the Helm release"
  type        = string
  default     = "nginx-release"
}

variable "helm_namespace" {
  description = "The Kubernetes namespace where the Helm chart should be deployed"
  type        = string
  default     = "default"
}

variable "helm_repo" {
  description = "The URL of the Helm chart repository"
  type        = string
  default     = "https://charts.bitnami.com/bitnami"
}

variable "helm_values" {
  description = "Optional Helm chart values provided as a raw YAML string"
  type        = string
  default     = ""
}

variable "portainer_skip_ssl_verify" {
  description = "Set to true to skip TLS certificate verification (useful for self-signed certs)"
  type        = bool
  default     = true
}
