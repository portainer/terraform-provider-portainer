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

variable "do_credential_id" {
  type    = number
  default = 1
}

variable "do_name" {
  type    = string
  default = "do-dev-cluster"
}

variable "do_region" {
  type    = string
  default = "nyc1"
}

variable "do_node_count" {
  type    = number
  default = 3
}

variable "do_node_size" {
  type    = string
  default = "s-2vcpu-4gb"
}

variable "do_network_id" {
  type    = string
  default = "1234-abcd"
}

variable "do_kubernetes_version" {
  type    = string
  default = "1.25.0"
}

variable "do_group_id" {
  type    = number
  default = 1
}

variable "do_stack_name" {
  type    = string
  default = "dev"
}

variable "do_tag_ids" {
  type    = list(number)
  default = [1]
}
