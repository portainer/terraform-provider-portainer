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

variable "edge_schedule_name" {
  type    = string
  default = "scheduled-edge-update"
}

variable "agent_image" {
  type    = string
  default = "portainer/agent:2.19.0"
}

variable "updater_image" {
  type    = string
  default = "portainer/portainer-updater:2.19.0"
}

variable "registry_id" {
  type    = number
  default = 1
}

variable "scheduled_time" {
  type        = string
  default     = "2025-05-10T10:00:00Z"
  description = "RFC3339 formatted time for update (UTC)"
}

variable "edge_group_ids" {
  type    = list(number)
  default = [1]
}

variable "update_type" {
  type        = number
  default     = 0
  description = "0 = update, 1 = rollback"
}
