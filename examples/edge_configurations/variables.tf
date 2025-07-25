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

variable "edge_config_name" {
  type        = string
  description = "Name of the Edge configuration"
  default     = "Test Edge Config"
}

variable "edge_config_type" {
  type    = string
  default = "general"
}

variable "edge_config_category" {
  type    = string
  default = "configuration"
}

variable "edge_config_base_dir" {
  type    = string
  default = "/etc/some/path/of/edge/config"
}

variable "edge_group_ids" {
  type    = list(number)
  default = [1]
}

variable "edge_config_file_path" {
  type    = string
  default = "config.zip"
}