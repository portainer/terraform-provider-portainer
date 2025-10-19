# 🌐 **Resource Documentation: `portainer_registry`**

# portainer_registry
The `portainer_registry` resource allows you to register container registries in Portainer.

## Example Usage

### Quay.io Registry
```hcl
resource "portainer_registry" "quay" {
  name                   = "Quay"
  url                    = "quay.io"
  type                   = 1
  authentication         = true
  username               = "quay-user"
  password               = "quay-token"
  quay_use_organisation  = true             # optional
  quay_organisation_name = "myorg"          # optional, but if define/exist quay_use_organisation you must define also value fot this variable
}
```

### Azure Registry
```hcl
resource "portainer_registry" "azure" {
  name     = "Azure"
  url      = "myproject.azurecr.io"
  type     = 2
  username = "azure-user"
  password = "azure-password"
}
```

### Custom Registry
- **Anonymus:**
```hcl
resource "portainer_registry" "custom" {
  name           = "Custom Registry"
  url            = "your-registry.example.com"
  type           = 3
  authentication = false
}
```

- **Authentication:**
```hcl
resource "portainer_registry" "custom_auth" {
  name           = "Custom Registry"
  url            = "your-registry.example.com"
  type           = 3
  authentication = true
  username       = "custom-registry-user"
  password       = "custom-registry-password"
}
```

### GitLab Registry
```hcl
resource "portainer_registry" "gitlab" {
  name         = "GitLab Registry"
  url          = "registry.gitlab.com"
  type         = 4
  username     = "gitlab-user"
  password     = "gitlab-acces-token"
  instance_url = "https://gitlab.com"
}
```

### ProGet Registry
```hcl
resource "portainer_registry" "proget" {
  name     = "ProGet"
  url      = "proget.example.com/example-registry"
  base_url = "proget.example.com"
  type     = 5
  username = "proget-user"
  password = "proget-password"
}
```

### Docker Hub
```hcl
resource "portainer_registry" "dockerhub" {
  name     = "DockerHub"
  type     = 6
  url      = "docker.io"
  username = "docker-user"
  password = "docker-access-token"
}
```

### AWS ECR
- **Anonymus:**
```hcl
resource "portainer_registry" "ecr" {
  name           = "AWS ECR"
  url            = "123456789.dkr.ecr.us-east-1.amazonaws.com"
  type           = 7
  authentication = false
}
```

- **Authentication:**
```hcl
resource "portainer_registry" "ecr" {
  name           = "AWS ECR"
  url            = "123456789.dkr.ecr.us-east-1.amazonaws.com"
  type           = 7
  authentication = true
  username       = "aws-access-key"
  password       = "aws-secret-key"
  aws_region     = "us-east-1"
}
```

### GitHub
- **GitHub type registry use only on BE Poertainer GUI:**
```hcl
resource "portainer_registry" "github" {
  name                     = "GitHub Registry"
  type                     = 8
  url                      = "ghcr.io"
  authentication           = true
  username                 = "your-github-username"
  password                 = "your-github-access-token"
  github_use_organisation  = true                       # optional
  github_organisation_name = "myorg"                    # optional, but if define/exist github_use_organisation you must define also value fot this variable
}
```

- **Add GitHub registry by custom type on CE Portainer GUI**, but without options define `github_use_organisation` and `github_organisation_name`:
```hcl
resource "portainer_registry" "github_custom" {
  name           = "GitHub Registry"
  url            = "ghcr.io"
  type           = 3
  authentication = true
  username       = "your-github-username"
  password       = "your-github-access-token"
}
```

## Lifecycle & Behavior
Registries are updated if any of the arguments change.
- To delete a registry created via Terraform, simply run:
```hcl
terraform destroy
```

- To update a registry, modify attributes and run:
```hcl
terraform apply
```

## Arguments Reference
| Name                       | Type   | Required                      | Description                                                                                                                                            |
| -------------------------- | ------ | ----------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `name`                     | string | ✅ yes                         | Name of the registry                                                                                                                                   |
| `url`                      | string | ✅ yes                         | Registry access URL (e.g., `registry.example.com:5000`)                                                                                                |
| `base_url`                 | string | 🚫 optional                   | Base URL (for some types like Azure, Custom, ProGet)                                                                                                   |
| `type`                     | int    | ✅ yes                         | Registry type:<br>1 = Quay.io<br>2 = Azure<br>3 = Custom<br>4 = GitLab<br>5 = ProGet<br>6 = Docker Hub<br>7 = AWS ECR<br>8 = GitHub Container Registry |
| `authentication`           | bool   | 🚫 optional (default `false`) | Whether the registry requires authentication (Custom, ECR, GitHub only)                                                                                |
| `username`                 | string | 🚫 optional                   | Username for authentication (if applicable) - must be define if `authentication = true`                                                                |
| `password`                 | string | 🚫 optional                   | Password or token for authentication (if applicable)- must be define if `authentication = true`                                                        |
| `instance_url`             | string | 🚫 optional                   | GitLab instance URL (for type = 4)                                                                                                                     |
| `aws_region`               | string | 🚫 optional                   | AWS Region (for ECR, type = 7)                                                                                                                         |
| `github_use_organisation`  | bool   | 🚫 optional (default `false`) | Whether to use a GitHub organisation scope (for type = 8, GitHub registry)                                                                             |
| `github_organisation_name` | string | 🚫 optional                   | Name of the GitHub organisation (for type = 8, GitHub registry) - must be define if `github_use_organisation = true`                                   |
| `quay_use_organisation`    | bool   | 🚫 optional (default `false`) | Whether to use an organisation namespace for Quay.io (for type = 1)                                                                                    |
| `quay_organisation_name`   | string | 🚫 optional                   | Name of the Quay.io organisation (for type = 1) - must be define if `quay_use_organisation = true`                                                     |

## Attributes Reference

| Name | Description              |
|------|--------------------------|
| `id` | ID of the Portainer registry |
