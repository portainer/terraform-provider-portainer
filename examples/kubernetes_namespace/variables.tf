variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
  # default     = "http://localhost:9000"
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
  # default     = "your-api-key-from-portainer"
}

variable "environment_id" {
  description = "ID of the Portainer environment (Kubernetes endpoint)."
  type        = number
}

variable "namespace_name" {
  description = "Name of the Kubernetes namespace to create."
  type        = string
}

variable "namespace_owner" {
  description = "Owner label for the namespace."
  type        = string
  default     = ""
}

variable "namespace_annotations" {
  description = "Map of annotations to apply to the namespace."
  type        = map(string)
  default = {
    owner = "terraform"
    env   = "test"
  }
}

variable "namespace_resource_quota" {
  description = "CPU and memory resource quota for the namespace."
  type = object({
    cpu    = string
    memory = string
  })
  default = {
    cpu    = "800m"
    memory = "129Mi"
  }
}
