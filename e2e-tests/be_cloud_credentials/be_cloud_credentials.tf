resource "portainer_cloud_credentials" "be" {
  name           = var.name
  cloud_provider = "aws"
  credentials = {
    accessKeyId     = "AKIAE2EEXAMPLE"
    secretAccessKey = "e2e-example-secret"
    region          = "eu-central-1"
  }
}
