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

variable "portainer_exec_endpoint_id" {
  description = "Portainer endpoint ID (standalone or swarm)"
  type        = number
  # default     = 1
}

variable "portainer_exec_service_name" {
  description = "Name of the container (standalone) or service (swarm)"
  type        = string
  # default     = "your-container-name"
}

variable "portainer_exec_command" {
  description = "Command to execute inside the container"
  type        = string
  # default     = "your command"
}

variable "portainer_exec_user" {
  description = "User to run the command as (e.g. root, uid)"
  type        = string
  default     = "root"
}
