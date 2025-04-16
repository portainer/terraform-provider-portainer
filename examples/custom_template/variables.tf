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

variable "custom_template_title" {
  description = "Title of the custom template"
  type        = string
}

variable "custom_template_description" {
  description = "Description of the custom template"
  type        = string
}

variable "custom_template_note" {
  description = "Note that appears in the UI"
  type        = string
}

variable "custom_template_platform" {
  description = "Platform: 1 = linux, 2 = windows"
  type        = number
}

variable "custom_template_type" {
  description = "Stack type: 1 = swarm, 2 = compose, 3 = kubernetes"
  type        = number
}

variable "custom_template_edge" {
  description = "Whether this is an Edge template"
  type        = bool
  default     = false
}

variable "custom_template_is_compose" {
  description = "Is Compose format (true/false)"
  type        = bool
  default     = false
}

variable "custom_template_file_content" {
  description = "Inline file content for the template (YAML/Compose)"
  type        = string
}
