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

variable "edge_job_name" {
  description = "Name of the edge job"
  type        = string
  default     = "example-edge-job"
}

variable "edge_job_cron" {
  description = "Cron expression for edge job scheduling"
  type        = string
  default     = "0 * * * *"
}

variable "edge_job_edge_groups" {
  description = "List of edge group IDs"
  type        = list(number)
  default     = []
}

variable "edge_job_endpoints" {
  description = "List of environment (endpoint) IDs"
  type        = list(number)
  default     = []
}

variable "edge_job_recurring" {
  description = "Whether the edge job should be recurring"
  type        = bool
  default     = true
}

variable "edge_job_file_content" {
  description = "Script content to run on edge agents"
  type        = string
  default     = <<-EOT
    #!/bin/sh
    echo "Hello from Edge Job!"
  EOT
}