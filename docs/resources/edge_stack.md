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
  environment = {
    FOO  = "BAR"
    FOO2 = "test"
  }
}
```

### Create Edge Stack from file

```hcl
resource "portainer_edge_stack" "example_file" {
  name               = "nginx-edge-file"
  deployment_type    = 0
  edge_groups        = [1]
  stack_file_path    = "./templates/nginx.yml"
  environment = {
    FOO  = "BAR"
    FOO2 = "test"
  }
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
  environment = {
    FOO  = "BAR"
    FOO2 = "test"
  }
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
### Common Arguments
| Name                      | Type         | Required    | Description                                                                 |
| ------------------------- | ------------ | ----------- | --------------------------------------------------------------------------- |
| `name`                    | string       | ✅ yes      | Name of the Edge Stack                                                      |
| `deployment_type`         | int          | ✅ yes      | Deployment type: `0` = Docker Compose, `1` = Kubernetes                     |
| `edge_groups`             | list(number) | ✅ yes      | List of Edge Group IDs where the stack will be deployed                     |
| `environment`             | map(string)  | 🚫 optional | Environment variables (key-value pairs) passed to the stack at deployment time |
| `registries`              | list(number) | 🚫 optional | List of registry IDs (for pulling images)                                   |
| `use_manifest_namespaces` | bool         | 🚫 optional | For Kubernetes only – respect namespaces in the manifest (default: `false`) |

---

### For String-based Edge Stack
| Name                 | Type   | Required | Description                                          |
| -------------------- | ------ | -------- | ---------------------------------------------------- |
| `stack_file_content` | string | ✅ yes    | Inline Docker Compose or Kubernetes manifest content |

### For File-based Edge Stack
| Name              | Type   | Required | Description                                                                 |
|-------------------|--------|----------|-----------------------------------------------------------------------------|
| `stack_file_path` | string | ✅ yes   | Path to a local Compose or Kubernetes manifest file                         |
| `pre_pull_image`  | bool   | 🚫 optional | Whether to pre-pull images before deploy (default: `false`)              |
| `retry_deploy`    | bool   | 🚫 optional | Whether to retry deploy if first attempt fails (default: `false`)        |
| `dryrun`          | bool   | 🚫 optional | If true, validate but do not persist the Edge Stack (default: `false`)   |

### For Repository-based Edge Stack
| Name                        | Type   | Required    | Description                                                          |
| --------------------------- | ------ | ----------- | -------------------------------------------------------------------- |
| `repository_url`            | string | ✅ yes       | Git repository URL                                                  |
| `repository_reference_name` | string | 🚫 optional | Git reference (default: `refs/heads/main`)                           |
| `file_path_in_repository`   | string | 🚫 optional | Path to the manifest inside the repo (default: `docker-compose.yml`) |
| `git_repository_authentication` | bool   | 🚫 optional | Enable auth for Git repo (default: `false`)                      |
| `repository_username`       | string | 🚫 optional | Git username (if auth is enabled)                                    |
| `repository_password`       | string | 🚫 optional | Git password/token (if auth is enabled)                              |
| `stack_webhook`             | bool   | 🚫 optional | Enable GitOps webhook (default: `false`)                             |
| `update_interval`           | string | 🚫 optional | Polling interval (e.g. `30m`, `1h`)                                  |
| `force_update`              | bool   | 🚫 optional | Whether to force redeploy (default: `false`)                         |
| `pull_image`                | bool   | 🚫 optional | Pull latest image during update (default: `false`)                   |
| `relative_path`             | string | 🚫 optional | Enables relative path volumes (from Compose) and sets the `filesystemPath` |
| `repository_git_credential_id` | int | 🚫 optional | ID of the Git credentials to use (replaces username/password) |

## 🧮 Computed Outputs
| Name          | Description                     |
| ------------- | ------------------------------- |
| `webhook_id`  | GitOps webhook UUID             |
| `webhook_url` | Full URL to trigger the webhook |

> `Webhook` currently working only for Portainer BE edition

---

## Attributes Reference

| Name | Description |
|:-----|:------------|
| `id` | ID of the Edge Stack inside Portainer |
