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

variable "stack_name" {
  description = "Name of the stack"
  type        = string
  default     = "nginx-standalone-string"
}

variable "stack_deployment_type" {
  description = "Deployment type: standalone, swarm, or kubernetes"
  type        = string
  default     = "standalone"
}

variable "stack_method" {
  description = "Creation method: string, file, repository, or url"
  type        = string
  default     = "string"
}

variable "stack_endpoint_id" {
  description = "Portainer environment/endpoint ID"
  type        = number
  default     = 3
}

variable "stack_file_content" {
  description = "Inline Docker Compose file content"
  type        = string
  default     = <<-EOT
    version: "3"
    services:
      web:
        image: nginx
  EOT
}

variable "stack_env_name" {
  description = "Environment variable name"
  type        = string
  default     = "MY_VAR"
}

variable "stack_env_value" {
  description = "Environment variable value"
  type        = string
  default     = "value"
}
