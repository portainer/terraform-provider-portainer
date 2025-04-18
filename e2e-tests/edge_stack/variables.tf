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

variable "edge_stack_name" {
  description = "Name of the Portainer Edge Stack"
  type        = string
  default     = "example-edge-stack"
}

variable "edge_stack_file_content" {
  description = "Inline stack file content for the Edge Stack"
  type        = string
  default     = <<-EOT
    version: '3'
    services:
      hello-world:
        image: hello-world
  EOT
}

variable "edge_stack_deployment_type" {
  description = "Deployment type (0 = Compose, 1 = Kubernetes)"
  type        = number
  default     = 0
}

variable "edge_stack_edge_groups" {
  description = "List of Edge Group IDs"
  type        = list(number)
  default     = []
}

variable "edge_stack_registries" {
  description = "List of registry IDs"
  type        = list(number)
  default     = []
}

variable "edge_stack_use_manifest_namespaces" {
  description = "Whether to use manifest namespaces"
  type        = bool
  default     = false
}
