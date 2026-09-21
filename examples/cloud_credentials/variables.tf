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

variable "cloud_credentials_name" {
  description = "Name of the cloud credential (e.g., my-aws-creds)"
  type        = string
  default     = "example-aws-creds"
}

variable "cloud_credentials_provider" {
  description = "Cloud provider (e.g., aws, digitalocean, civo)"
  type        = string
  default     = "aws"
}

variable "cloud_credentials_data" {
  description = "Credentials for the cloud provider. The resource takes a map, not a JSON string."
  type        = map(string)
  sensitive   = true

  default = {
    accessKeyId     = "your-access-key"
    secretAccessKey = "your-secret-key"
    region          = "eu-central-1"
  }
}
