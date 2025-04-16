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

variable "s3_access_key" {
  description = "AWS or compatible S3 Access Key"
  type        = string
  sensitive   = true
}

variable "s3_secret_key" {
  description = "AWS or compatible S3 Secret Access Key"
  type        = string
  sensitive   = true
}

variable "s3_bucket" {
  description = "S3 bucket name where backups will be stored"
  type        = string
}

variable "s3_region" {
  description = "Region for S3 bucket (e.g., eu-central-1)"
  type        = string
  default     = "eu-central-1"
}

variable "s3_endpoint" {
  description = "S3-compatible endpoint URL"
  type        = string
}

variable "backup_password" {
  description = "Password used to encrypt the Portainer backup archive"
  type        = string
  sensitive   = true
}

variable "backup_cron_rule" {
  description = "Cron rule for scheduling the backup (e.g., '@daily')"
  type        = string
  default     = "@daily"
}
