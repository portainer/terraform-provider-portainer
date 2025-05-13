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

variable "custom_template_title" {
  description = "Title of the custom template"
  type        = string
  default     = "Portainer Agent"
}

variable "custom_template_description" {
  description = "Description of the custom template"
  type        = string
  default     = "Deploy Portainer Agent container"
}

variable "custom_template_note" {
  description = "Note that appears in the UI"
  type        = string
  default     = "Runs Portainer Agent container with required mounts"
}

variable "custom_template_platform" {
  description = "Platform: 1 = linux, 2 = windows"
  type        = number
  default     = 1
}

variable "custom_template_type" {
  description = "Stack type: 1 = swarm, 2 = compose, 3 = kubernetes"
  type        = number
  default     = 2
}

variable "custom_template_file_path" {
  description = "Inline file content for the template (YAML/Compose)"
  type        = string
  default     = "./portainer-agent.yml"
}

variable "portainer_skip_ssl_verify" {
  description = "Set to true to skip TLS certificate verification (useful for self-signed certs)"
  type        = bool
  default     = true
}
