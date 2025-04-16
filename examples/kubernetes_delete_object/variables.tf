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

variable "environment_id" {
  description = "ID of the Portainer environment (Kubernetes endpoint)."
  type        = number
}

variable "resource_type" {
  description = "Type of resource to delete (e.g. services, ingresses, jobs, cron_jobs, roles, role_bindings, service_accounts)."
  type        = string
  validation {
    condition     = contains(["services", "ingresses", "jobs", "cron_jobs", "roles", "role_bindings", "service_accounts"], var.resource_type)
    error_message = "Allowed values for resource_type are: services, ingresses, jobs, cron_jobs, roles, role_bindings, service_accounts."
  }
}

variable "namespace" {
  description = "Kubernetes namespace where the resources to be deleted reside."
  type        = string
}

variable "names" {
  description = "List of resource names to delete."
  type        = list(string)
}
