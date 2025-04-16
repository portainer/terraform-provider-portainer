variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
  # default     = "http://localhost:9000"
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
  # default     = "your-portainer-api-key"
}

variable "environment_id" {
  type        = number
  description = "Portainer environment (endpoint) ID"
  default     = 4
}

variable "namespace" {
  type        = string
  description = "Kubernetes namespace where the ingress will be created"
  default     = "default"
}

variable "ingress_name" {
  type        = string
  description = "Name of the ingress resource"
  default     = "example-ingress"
}

variable "class_name" {
  type        = string
  description = "Ingress controller class name (e.g., nginx)"
  default     = "nginx"
}

variable "annotations" {
  type        = map(string)
  description = "Annotations to be applied to the ingress"
  default = {
    "kubernetes.io/ingress.class" = "nginx"
  }
}

variable "labels" {
  type        = map(string)
  description = "Labels to be applied to the ingress"
  default = {
    "app" = "nginx"
  }
}

variable "hosts" {
  type        = list(string)
  description = "List of hostnames for the ingress"
  default     = ["example.com"]
}

variable "tls_hosts" {
  type        = list(string)
  description = "List of TLS hosts"
  default     = ["example.com"]
}

variable "tls_secret_name" {
  type        = string
  description = "Secret name for TLS"
  default     = "example-tls"
}

variable "path_host" {
  type        = string
  description = "Host for ingress path"
  default     = "example.com"
}

variable "path" {
  type        = string
  description = "Ingress path"
  default     = "/"
}

variable "path_type" {
  type        = string
  description = "Type of the path (e.g., Prefix)"
  default     = "Prefix"
}

variable "service_port" {
  type        = number
  description = "Port number for the service"
  default     = 80
}

variable "service_name" {
  type        = string
  description = "Name of the Kubernetes service"
  default     = "nginx-service"
}
