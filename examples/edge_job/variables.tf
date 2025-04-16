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

variable "edge_job_name" {
  type        = string
  description = "Name of the edge job"
}

variable "edge_job_cron" {
  type        = string
  description = "Cron expression for edge job scheduling"
}

variable "edge_job_edge_groups" {
  type        = list(number)
  description = "List of edge group IDs"
}

variable "edge_job_endpoints" {
  type        = list(number)
  description = "List of environment (endpoint) IDs"
}

variable "edge_job_recurring" {
  type = bool
  # default     = true
  description = "Whether the edge job should be recurring"
}

variable "edge_job_file_content" {
  type        = string
  description = "Script content to run on edge agents"
}
