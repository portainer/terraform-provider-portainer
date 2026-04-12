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

variable "user_id" {
  description = "ID of the user who owns the git credential"
  type        = number
  default     = 1
}

variable "git_credential_name" {
  description = "Name of the user git credential"
  type        = string
  default     = "my-git-cred"
}

variable "git_credential_username" {
  description = "Username for the git credential"
  type        = string
  default     = "gituser"
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
