variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
  default     = "https://localhost:9443"
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
  default     = "ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8="
}

variable "portainer_skip_ssl_verify" {
  description = "Set to true to skip TLS certificate verification (useful for self-signed certs)"
  type        = bool
  default     = true
}

variable "endpoint_id" {
  description = "ID of the Portainer environment (Docker Swarm cluster) where the node is located"
  type        = number
  default     = 1
}

variable "docker_node_hostname" {
  description = "Hostname of the Docker Swarm node to look up"
  type        = string
  default     = "node-1"
}
