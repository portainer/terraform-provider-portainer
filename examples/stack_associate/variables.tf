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

variable "stack_associate_stack_id" {
  description = "ID of the orphaned stack to associate"
  type        = number
  default     = 12
}

variable "stack_associate_endpoint_id" {
  description = "ID of the environment (endpoint) to associate the stack with"
  type        = number
  default     = 1
}

variable "stack_associate_swarm_id" {
  description = "ID of the Swarm cluster where the stack should be associated"
  type        = string
  default     = "jpofkc0i9uo9wtx1zesuk649w"
}

variable "stack_associate_orphaned_running" {
  description = "Whether the stack is an orphaned running stack"
  type        = bool
  default     = true
}
