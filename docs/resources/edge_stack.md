# 🧩 Resource Documentation: `portainer_edge_stack`

## Overview

The `portainer_edge_stack` resource allows you to manage **Portainer Edge Stacks**. 
You can create Edge Stacks using:
- Inline content (`stack_file_content`)
- Local file (`stack_file_path`)
- Git repository (`repository_url`)

---

## Example Usage

### Create Edge Stack from `stack_file_content`

```hcl
resource "portainer_edge_stack" "example_string" {
  name                  = "nginx-edge-string"
  deployment_type       = 0 # 0 = Compose, 1 = Kubernetes
  edge_groups           = [1]
  stack_file_content    = <<-EOT
    version: "3"
    services:
      web:
        image: nginx
  EOT
}
```

### Create Edge Stack from file

```hcl
resource "portainer_edge_stack" "example_file" {
  name               = "nginx-edge-file"
  deployment_type    = 0
  edge_groups        = [1]
  stack_file_path    = "./templates/nginx.yml"
}
```

### Create Edge Stack from Git repository

```hcl
resource "portainer_edge_stack" "example_git" {
  name                     = "nginx-edge-git"
  deployment_type          = 0
  edge_groups              = [1]
  repository_url           = "https://github.com/example/repo"
  repository_username      = "gituser"
  repository_password      = "supersecret"
  repository_reference_name = "refs/heads/main"
  file_path_in_repository  = "docker-compose.yml"
}
```

---

## Lifecycle & Behavior

- **To delete an Edge Stack:**

```bash
terraform destroy
```

- **To update an Edge Stack:** Modify any attributes and re-apply:

```bash
terraform apply
```

> **⚠️ Important:** One of `stack_file_content`, `stack_file_path`, or `repository_url` must be provided.

---

## Arguments Reference

| Name | Type | Required | Description |
|:-----|:-----|:--------:|:------------|
| `name` | string | ✅ yes | Name of the Edge Stack |
| `deployment_type` | int | ✅ yes | Deployment type: `0` = Docker Compose, `1` = Kubernetes |
| `edge_groups` | list(number) | ✅ yes | List of Edge Group IDs where the stack will be deployed |
| `stack_file_content` | string | 🚫 optional | Inline Compose or Kubernetes manifest content |
| `stack_file_path` | string | 🚫 optional | Path to a Compose or Kubernetes file |
| `repository_url` | string | 🚫 optional | Git repository URL |
| `repository_username` | string | 🚫 optional | Git repository username |
| `repository_password` | string (sensitive) | 🚫 optional | Git repository password or token |
| `repository_reference_name` | string | 🚫 optional | Git reference (branch/tag); default is `refs/heads/main` |
| `file_path_in_repository` | string | 🚫 optional | Path to Compose/Kubernetes file inside the repo; default `docker-compose.yml` |
| `registries` | list(number) | 🚫 optional | List of registry IDs linked to the stack (for pulling images) |
| `use_manifest_namespaces` | bool | 🚫 optional | (Kubernetes only) Respect namespace defined in manifest (default: `false`) |

---

## Attributes Reference

| Name | Description |
|:-----|:------------|
| `id` | ID of the Edge Stack inside Portainer |

---
