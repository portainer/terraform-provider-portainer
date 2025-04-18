variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
  # default     = "http://localhost:9000"
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
  # default     = "your-api-key"
}

variable "docker_node_endpoint_id" {
  description = "ID of the Portainer endpoint (environment)"
  type        = number
  default     = 1
}

variable "docker_node_id" {
  description = "ID of the Docker Swarm node"
  type        = string
  default     = "wna048ajhbc1n1t5ispvf6mvg"
}

variable "docker_node_version" {
  description = "Swarm node version required for update/delete"
  type        = number
  default     = 4869
}

variable "docker_node_name" {
  description = "Custom name of the node"
  type        = string
  default     = "node-name"
}

variable "docker_node_availability" {
  description = "Availability of the node (active, pause, drain)"
  type        = string
  default     = "active"
}

variable "docker_node_role" {
  description = "Role of the node (manager or worker)"
  type        = string
  default     = "manager"
}

variable "docker_node_labels" {
  description = "Map of node labels"
  type        = map(string)
  default = {
    foo = "barrerun"
  }
}
