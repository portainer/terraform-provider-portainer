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
  default     = "nginx-deploy"
  description = "Name of the Edge configuration"
}

variable "edge_config_type" {
  type    = string
  default = "file"
}

variable "edge_config_category" {
  type    = string
  default = "infrastructure"
}

variable "edge_config_base_dir" {
  type    = string
  default = "/opt/nginx"
}

variable "edge_group_ids" {
  type    = list(number)
  default = [1]
}

variable "edge_config_file_path" {
  type    = string
  default = "nginx.yaml"
}

variable "edge_config_state" {
  type        = number
  default     = 2
  description = "Desired state to set via /edge_configurations/{id}/{state}"
}

