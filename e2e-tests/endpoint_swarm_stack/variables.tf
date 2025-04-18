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
  description = "Name of the Portainer stack"
  type        = string
  default     = "your-swarm-string"
}

variable "stack_deployment_type" {
  description = "Deployment type of the stack (e.g., 'swarm')"
  type        = string
  default     = "swarm"
}

variable "stack_method" {
  description = "Method used to deploy the stack (e.g., 'string', 'repository')"
  type        = string
  default     = "string"
}

variable "stack_endpoint_id" {
  description = "ID of the Portainer endpoint"
  type        = number
  default     = 3
}

variable "stack_file_content" {
  description = "The content of the docker-compose file"
  type        = string
  default     = <<-EOT
    version: "3"
    services:
      web:
        image: nginx
  EOT
}

variable "stack_env_name" {
  description = "Name of the environment variable"
  type        = string
  default     = "MY_VAR"
}

variable "stack_env_value" {
  description = "Value of the environment variable"
  type        = string
  default     = "value"
}
