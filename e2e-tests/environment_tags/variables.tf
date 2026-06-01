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

variable "portainer_environment_name" {
  description = "Portainer environment name (with tags)"
  type        = string
  default     = "env-with-tags"
}

variable "portainer_environment_address" {
  description = "Portainer environment address. Defaults to the local Docker socket so the e2e test does not depend on a reachable agent."
  type        = string
  default     = "unix:///var/run/docker.sock"
}

variable "portainer_environment_type" {
  description = "Environment type: 1 = Docker, 2 = Agent, 3 = Azure, 4 = Edge Agent, 5 = Kubernetes, 6 = Kubernetes via agent."
  type        = number
  default     = 1
}
