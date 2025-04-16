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

variable "endpoint_id" {
  description = "ID of the environment where the network will be created"
  type        = number
  default     = 3
}

variable "network_name" {
  description = "Name of the Docker network"
  type        = string
  default     = "your-network"
}

variable "network_driver" {
  description = "Network driver (bridge, overlay, macvlan, etc.)"
  type        = string
  default     = "bridge"
}

variable "network_internal" {
  description = "Whether the network is internal"
  type        = bool
  default     = false
}

variable "network_attachable" {
  description = "Whether containers can be attached manually"
  type        = bool
  default     = false
}

variable "network_ingress" {
  description = "Whether it's an ingress network"
  type        = bool
  default     = false
}

variable "network_config_only" {
  description = "If this network is only configuration"
  type        = bool
  default     = false
}

variable "network_config_from" {
  description = "Name of another config-only network to inherit from"
  type        = string
  default     = ""
}

variable "network_enable_ipv4" {
  description = "Enable IPv4 networking"
  type        = bool
  default     = true
}

variable "network_enable_ipv6" {
  description = "Enable IPv6 networking"
  type        = bool
  default     = false
}

variable "network_options" {
  description = "Driver-specific options"
  type        = map(string)
  default = {
    "com.docker.network.bridge.enable_icc"           = "true"
    "com.docker.network.bridge.enable_ip_masquerade" = "true"
  }
}

variable "network_labels" {
  description = "Labels to apply to the network"
  type        = map(string)
  default = {
    "env"     = "test"
    "purpose" = "terraform"
  }
}
