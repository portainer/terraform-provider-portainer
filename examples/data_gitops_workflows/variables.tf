variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
}

variable "git_token" {
  description = "Personal access token used to authenticate against the Git repository"
  type        = string
  sensitive   = true
  default     = ""
}
