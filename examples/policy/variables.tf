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

variable "policy_name" {
  description = "Name of the policy"
  type        = string
  default     = "test-policy"
}

variable "policy_environment_type" {
  description = "Environment type for the policy (e.g., kubernetes)"
  type        = string
  default     = "kubernetes"
}

variable "policy_type" {
  description = "Type of the policy (e.g., security)"
  type        = string
  default     = "security"
}

variable "policy_environment_groups" {
  description = "List of environment group IDs to associate with the policy"
  type        = list(number)
  default     = [1]
}

variable "policy_data" {
  description = "JSON-encoded policy data"
  type        = string
  default     = "{\"restrictDefaultNamespace\":true}"
}
