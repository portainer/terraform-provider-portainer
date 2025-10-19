# AWS ECR (Anonymous)
resource "portainer_registry" "ecr_anonymous" {
  name           = var.ecr_anonymous_name
  url            = var.ecr_anonymous_url
  type           = 7
  authentication = false
}

# AWS ECR (Authentication)
resource "portainer_registry" "ecr" {
  name           = var.ecr_name
  url            = var.ecr_url
  type           = 7
  authentication = true
  username       = var.ecr_username
  password       = var.ecr_password
  aws_region     = var.ecr_region
}
