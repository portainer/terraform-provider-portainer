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

variable "edge_group_name" {
  description = "Name of the edge group"
  type        = string
  default     = "static-group"
}

variable "edge_group_dynamic" {
  description = "Whether the edge group is dynamic"
  type        = bool
  default     = false
}

variable "edge_group_partial_match" {
  description = "Whether to use partial match when dynamic = true"
  type        = bool
  default     = false
}

variable "edge_group_tag_ids" {
  description = "List of tag IDs used for dynamic matching"
  type        = list(number)
  default     = [] # Replace with actual tag IDs
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

variable "edge_stack_registries" {
  description = "List of registry IDs"
  type        = list(number)
  default     = []
}

variable "portainer_skip_ssl_verify" {
  description = "Set to true to skip TLS certificate verification (useful for self-signed certs)"
  type        = bool
  default     = true
}
