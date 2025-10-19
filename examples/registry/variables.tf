#############################################
# Provider
#############################################

variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
  # default     = "https://localhost:9443"
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
  # default     = "some-your-api-token"
}

variable "portainer_skip_ssl_verify" {
  description = "Set to true to skip TLS certificate verification (useful for self-signed certs)"
  type        = bool
  default     = true
}

#############################################
# Quay.io
#############################################

variable "quay_name" {
  description = "Name of the Quay.io registry."
  type        = string
  default     = "Quay"
}

variable "quay_url" {
  description = "URL of the Quay.io registry."
  type        = string
  default     = "quay.io"
}

variable "quay_authentication" {
  description = "Enable authentication for Quay.io registry."
  type        = bool
  default     = true
}

variable "quay_username" {
  description = "Username for Quay.io authentication."
  type        = string
  default     = "quay-user"
}

variable "quay_password" {
  description = "Access token or password for Quay.io registry."
  type        = string
  sensitive   = true
  default     = "quay-token"
}

variable "quay_use_organisation" {
  description = "Whether to use organisation namespace for Quay.io registry."
  type        = bool
  default     = true
}

variable "quay_organisation_name" {
  description = "Name of the Quay.io organisation (required if quay_use_organisation = true)."
  type        = string
  default     = "myorg"
}


#############################################
# Azure
#############################################

variable "azure_name" {
  description = "Name of the Azure Container Registry."
  type        = string
  default     = "Azure"
}

variable "azure_url" {
  description = "Azure Container Registry URL."
  type        = string
  default     = "myproject.azurecr.io"
}

variable "azure_username" {
  description = "Username for Azure Container Registry."
  type        = string
  default     = "azure-user"
}

variable "azure_password" {
  description = "Password or access key for Azure Container Registry."
  type        = string
  sensitive   = true
  default     = "azure-password"
}


#############################################
# Custom Registries
#############################################

variable "custom_name" {
  description = "Name of the anonymous custom registry."
  type        = string
  default     = "Custom Registry"
}

variable "custom_url" {
  description = "URL of the anonymous custom registry."
  type        = string
  default     = "your-registry.example.com"
}

variable "custom_authentication" {
  description = "Whether authentication is required for the custom registry."
  type        = bool
  default     = false
}

variable "custom_auth_name" {
  description = "Name of the authenticated custom registry."
  type        = string
  default     = "Custom Registry Auth"
}

variable "custom_auth_url" {
  description = "URL of the authenticated custom registry."
  type        = string
  default     = "your-registry.example.com"
}

variable "custom_auth_authentication" {
  description = "Whether authentication is required for the authenticated custom registry."
  type        = bool
  default     = true
}

variable "custom_auth_username" {
  description = "Username for the authenticated custom registry."
  type        = string
  default     = "custom-registry-user"
}

variable "custom_auth_password" {
  description = "Password or token for the authenticated custom registry."
  type        = string
  sensitive   = true
  default     = "custom-registry-password"
}


#############################################
# GitLab
#############################################

variable "gitlab_name" {
  description = "Name of the GitLab registry."
  type        = string
  default     = "GitLab Registry"
}

variable "gitlab_url" {
  description = "URL of the GitLab registry."
  type        = string
  default     = "registry.gitlab.com"
}

variable "gitlab_username" {
  description = "Username for the GitLab registry."
  type        = string
  default     = "gitlab-user"
}

variable "gitlab_password" {
  description = "Access token or password for the GitLab registry."
  type        = string
  sensitive   = true
  default     = "gitlab-access-token"
}

variable "gitlab_instance_url" {
  description = "GitLab instance URL."
  type        = string
  default     = "https://gitlab.com"
}


#############################################
# ProGet
#############################################

variable "proget_name" {
  description = "Name of the ProGet registry."
  type        = string
  default     = "ProGet"
}

variable "proget_url" {
  description = "Full registry URL of the ProGet registry."
  type        = string
  default     = "proget.example.com/example-registry"
}

variable "proget_base_url" {
  description = "Base URL of the ProGet registry."
  type        = string
  default     = "proget.example.com"
}

variable "proget_username" {
  description = "Username for ProGet authentication."
  type        = string
  default     = "proget-user"
}

variable "proget_password" {
  description = "Password or API token for ProGet registry."
  type        = string
  sensitive   = true
  default     = "proget-password"
}


#############################################
# Docker Hub
#############################################

variable "dockerhub_name" {
  description = "Name of the Docker Hub registry."
  type        = string
  default     = "DockerHub"
}

variable "dockerhub_url" {
  description = "URL of the Docker Hub registry."
  type        = string
  default     = "docker.io"
}

variable "dockerhub_username" {
  description = "Docker Hub username."
  type        = string
  default     = "docker-user"
}

variable "dockerhub_password" {
  description = "Docker Hub access token or password."
  type        = string
  sensitive   = true
  default     = "docker-access-token"
}


#############################################
# AWS ECR
#############################################

variable "ecr_anonymous_name" {
  description = "Name of the anonymous AWS ECR registry."
  type        = string
  default     = "AWS ECR Anonymous"
}

variable "ecr_anonymous_url" {
  description = "URL of the anonymous AWS ECR registry."
  type        = string
  default     = "123456789.dkr.ecr.us-east-1.amazonaws.com"
}

variable "ecr_name" {
  description = "Name of the authenticated AWS ECR registry."
  type        = string
  default     = "AWS ECR"
}

variable "ecr_url" {
  description = "URL of the authenticated AWS ECR registry."
  type        = string
  default     = "123456789.dkr.ecr.us-east-1.amazonaws.com"
}

variable "ecr_username" {
  description = "AWS access key for ECR authentication."
  type        = string
  default     = "aws-access-key"
}

variable "ecr_password" {
  description = "AWS secret key for ECR authentication."
  type        = string
  sensitive   = true
  default     = "aws-secret-key"
}

variable "ecr_region" {
  description = "AWS region where the ECR registry is hosted."
  type        = string
  default     = "us-east-1"
}


#############################################
# GitHub
#############################################

variable "github_name" {
  description = "Name of the GitHub Container Registry."
  type        = string
  default     = "GitHub Registry"
}

variable "github_url" {
  description = "URL of the GitHub Container Registry."
  type        = string
  default     = "ghcr.io"
}

variable "github_authentication" {
  description = "Enable authentication for GitHub Container Registry."
  type        = bool
  default     = true
}

variable "github_username" {
  description = "GitHub username used for registry authentication."
  type        = string
  default     = "your-github-username"
}

variable "github_password" {
  description = "GitHub Personal Access Token used for registry authentication."
  type        = string
  sensitive   = true
  default     = "your-github-access-token"
}

variable "github_use_organisation" {
  description = "Whether to use organisation namespace for GitHub registry."
  type        = bool
  default     = true
}

variable "github_organisation_name" {
  description = "Name of the GitHub organisation (required if github_use_organisation = true)."
  type        = string
  default     = "myorg"
}

variable "github_custom_name" {
  description = "Name of the GitHub registry (custom CE workaround)."
  type        = string
  default     = "GitHub Registry Custom"
}

variable "github_custom_url" {
  description = "URL of the GitHub registry (custom CE workaround)."
  type        = string
  default     = "ghcr.io"
}

variable "github_custom_authentication" {
  description = "Enable authentication for GitHub custom CE registry."
  type        = bool
  default     = true
}

variable "github_custom_username" {
  description = "GitHub username for the custom CE registry."
  type        = string
  default     = "your-github-username"
}

variable "github_custom_password" {
  description = "GitHub Personal Access Token for the custom CE registry."
  type        = string
  sensitive   = true
  default     = "your-github-access-token"
}
