variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
  # default     = "https://localhost:9443"
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
  # default     = "your-api-key-from-portainer"
}

variable "git_credential_name" {
  description = "Name of the shared git credential"
  type        = string
  default     = "test-git-cred"
}

variable "git_credential_username" {
  description = "Username for the git credential"
  type        = string
  default     = "testuser"
}

variable "git_credential_password" {
  description = "Password or token for the git credential"
  type        = string
  sensitive   = true
  # default     = "your-git-token"
}

variable "git_credential_authorization_type" {
  description = "Authorization type (0 = basic)"
  type        = number
  default     = 0
}

variable "git_credential_url_pattern" {
  description = "Repository URL pattern the credential is offered for (e.g. github.com/my-org/*); required by Portainer Business Edition 3.1 and later, not supported before 2.45"
  type        = string
  default     = null
}
