# 🚀 **Resource Documentation: `portainer_stack`**

# portainer_stack
The `portainer_stack` resource allows you to manage application stacks in Portainer.
You can deploy stacks in `standalone`, `swarm`, or `kubernetes` environments and choose to define the stack via `string`, `file`, `repository`, or `url` (for K8s only).

## Example Usage

### Deploy Standalone Stack from String
```hcl
resource "portainer_stack" "standalone_string" {
  name              = "your-standalone"
  deployment_type   = "standalone"
  method            = "string"
  endpoint_id       = 1

  stack_file_content = <<-EOT
    version: "3"
    services:
      web:
        image: nginx
  EOT

  env {
    name  = "MY_VAR"
    value = "value"
  }
}
```
### Deploy Standalone Stack from File
```hcl
resource "portainer_stack" "standalone_file" {
  name              = "your-standalone-file"
  deployment_type   = "standalone"
  method            = "file"
  endpoint_id       = 1
  stack_file_path   = "./docker-compose.yml"

  env {
    name  = "MY_VAR"
    value = "value"
  }
}
```
### Deploy Standalone Stack from Git Repository
```hcl
resource "portainer_stack" "standalone_repo" {
  name                      = "your-standalone-repo"
  deployment_type           = "standalone"
  method                    = "repository"
  endpoint_id               = 1
  repository_url            = "https://github.com/example/repo"
  repository_reference_name = "refs/heads/main"
  file_path_in_repository   = "docker-compose.yml"
  tlsskip_verify            = false
  additional_files            = [
    "some-other-docker-compose.yml"
  ]
  
  # Optional GitOps enhancements:
  stack_webhook             = true                      # Enables GitOps webhook
  update_interval           = "5m"                       # Auto-update interval
  pull_image                = true                       # Pull latest image on update
  force_update              = true                       # Prune services not in compose file
  git_repository_authentication = true                   # If authentication is required
  repository_username       = "gituser"
  repository_password       = "secure"
}

output "webhook_url" {
  description = "GitOps webhook trigger URL"
  value       = portainer_stack.standalone_repo.webhook_url
}
```
### Deploy Swarm Stack from String
```hcl
resource "portainer_stack" "swarm_string" {
  name            = "your-swarm-string"
  deployment_type = "swarm"
  method          = "string"
  endpoint_id     = 1

  stack_file_content = <<-EOT
    version: "3"
    services:
      web:
        image: nginx
  EOT

  env {
    name  = "MY_VAR"
    value = "value"
  }
}
```
### Deploy Swarm Stack from File
```hcl
resource "portainer_stack" "swarm_file" {
  name            = "your-swarm"
  deployment_type = "swarm"
  method          = "file"
  endpoint_id     = 1
  stack_file_path = "./docker-compose.yml"

  env {
    name  = "MY_VAR"
    value = "value"
  }
}
```
### Deploy Swarm Stack from Git Repository
```hcl
resource "portainer_stack" "swarm_repo" {
  name                      = "your-swarm-repo"
  deployment_type           = "swarm"
  method                    = "repository"
  endpoint_id               = 1
  repository_url            = "https://github.com/example/repo"
  repository_username       = "gituser"
  repository_password       = "secure"
  repository_reference_name = "refs/heads/main"
  file_path_in_repository   = "docker-compose.yml"
  additional_files            = [
    "some-other-docker-compose.yml"
  ]
}
```
### Deploy Kubernetes Stack from String
```hcl
resource "portainer_stack" "k8s_string" {
  name              = "k8s-inline"
  deployment_type   = "kubernetes"
  method            = "string"
  endpoint_id       = 2
  namespace         = "default"
  stack_file_content = <<-EOT
    apiVersion: v1
    kind: Pod
    metadata:
      name: nginx
    spec:
      containers:
      - name: nginx
        image: nginx
  EOT
}
```
### Deploy Kubernetes Stack from Repository
```hcl
resource "portainer_stack" "k8s_repo" {
  name                      = "kube-stack"
  deployment_type           = "kubernetes"
  method                    = "repository"
  endpoint_id               = 2
  repository_url            = "https://github.com/example/repo"
  repository_username       = "gituser"
  repository_password       = "secure"
  repository_reference_name = "refs/heads/main"
  file_path_in_repository   = "kube.yaml"
  namespace                 = "default"
  compose_format            = true
  additional_files            = [
    "some-other-file.yaml"
  ]
}
```
### Deploy Kubernetes Stack from URL
```hcl
resource "portainer_stack" "k8s_url" {
  name            = "k8s-url"
  deployment_type = "kubernetes"
  method          = "url"
  endpoint_id     = 2
  namespace       = "default"
  manifest_url    = "https://raw.githubusercontent.com/example/nginx.yaml"
  compose_format  = false
}
```
---

## Lifecycle & Behavior
- To delete a custom teplate:
```hcl
terraform destroy
```

- To update a template, change any of the attributes and re-apply:
```hcl
terraform apply
```
> ⚠️ **One of `stack_file_content`, `stack_file_path`, `repository_url`, or `manifest_url` (for K8s) must be provided depending on the method.**

---

## Arguments Reference
### Common Arguments
| Name              | Type         | Required    | Description                                                          |
| ----------------- | ------------ | ----------- | -------------------------------------------------------------------- |
| `name`            | string       | ✅ yes       | Name of the stack                                                   |
| `deployment_type` | string       | ✅ yes       | One of: `standalone`, `swarm`, `kubernetes`                         |
| `method`          | string       | ✅ yes       | Creation method: `string`, `file`, `repository`, or `url` (K8s only)|
| `endpoint_id`     | int          | ✅ yes       | ID of the environment where the stack will be deployed              |
| `env`             | list(object) | 🚫 optional | List of environment variables (`name`, `value`)                      |
| `prune`           | bool         | 🚫 optional | Remove services no longer in stack definition (default: `false`)     |
| `pull_image`      | bool         | 🚫 optional | Pull latest image during update (default: `false`)                   |

---
### 🐳 Docker Stack (standalone/swarm)
#### Method: `string`
| Name                 | Type   | Required | Description                   |
| -------------------- | ------ | -------- | ----------------------------- |
| `stack_file_content` | string | ✅ yes    | Inline Docker Compose content|

#### Method: `file`
| Name              | Type   | Required | Description                       |
| ----------------- | ------ | -------- | --------------------------------- |
| `stack_file_path` | string | ✅ yes    | Path to local Docker Compose file|

#### Method: `repository`
| Name                            | Type   | Required    | Description                                          |
| ------------------------------- | ------ | ----------- | ---------------------------------------------------- |
| `repository_url`                | string | ✅ yes      | Git repository URL                                   |
| `repository_reference_name`     | string | ✅ yes      | Git reference (default: `refs/heads/main`)           |
| `file_path_in_repository`       | string | ✅ yes      | Path to Compose file (default: `docker-compose.yml`) |
| `tlsskip_verify`                | bool   | 🚫 optional | Skip TLS verification (default: `false`)             |
| `git_repository_authentication` | bool   | 🚫 optional | Enable auth for Git repo (default: `false`)          |
| `repository_username`           | string | 🚫 optional | Git username (if auth is enabled)                    |
| `repository_password`           | string | 🚫 optional | Git password/token (if auth is enabled)              |
| `stack_webhook`                 | bool   | 🚫 optional | Enable GitOps webhook (default: `false`)             |
| `update_interval`               | string | 🚫 optional | Polling interval (e.g. `30m`, `1h`)                  |
| `force_update`                  | bool   | 🚫 optional | Whether to force redeploy (default: `false`)         |
| `support_relative_path`         | bool   | 🚫 optional | Enable resolving of relative paths (default: `false`)|
| `filesystem_path`               | string | 🚫 optional | Base path on disk to resolve relative paths from     |
| `additional_files`              | string | 🚫 optional | List of additional yaml files paths                  |

#### Extra for `swarm`
| Name       | Type   | Required    | Description                  |
| ---------- | ------ | ----------- | ---------------------------- |
| `swarm_id` | string | ✅ yes      | Auto-filled if not specified |

---

### ☸️ Kubernetes Stack
#### Method: `string`
| Name                 | Type   | Required    | Description                           |
| -------------------- | ------ | ----------- | ------------------------------------- |
| `stack_file_content` | string | ✅ yes       | Inline Kubernetes manifest (YAML)    |
| `namespace`          | string | ✅ yes       | Target namespace                     |
| `compose_format`     | bool   | 🚫 optional | Use Compose format (default: `false`) |

#### Method: `repository`
| Name                            | Type   | Required    | Description                                      |
| ------------------------------- | ------ | ----------- | ------------------------------------------------ |
| `repository_url`                | string | ✅ yes      | Git repository URL                               |
| `namespace`                     | string | ✅ yes      | Kubernetes namespace                             |
| `repository_reference_name`     | string | ✅ yes      | Git reference (default: `refs/heads/main`)       |
| `file_path_in_repository`       | string | ✅ yes      | Path to manifest (default: `docker-compose.yml`) |
| `tlsskip_verify`                | bool   | 🚫 optional | Skip TLS verification (default: `false`)         |
| `git_repository_authentication` | bool   | 🚫 optional | Enable auth for Git repo (default: `false`)      |
| `repository_username`           | string | 🚫 optional | Git username (if auth is enabled)                |
| `repository_password`           | string | 🚫 optional | Git password/token (if auth is enabled)          |
| `stack_webhook`                 | bool   | 🚫 optional | Enable GitOps webhook (default: `false`)         |
| `update_interval`               | string | 🚫 optional | Polling interval (e.g. `30m`, `1h`)              |
| `force_update`                  | bool   | 🚫 optional | Whether to force redeploy (default: `false`)     |
| `compose_format`                | bool   | 🚫 optional | Compose format support (default: `false`)        |
| `additional_files`              | string | 🚫 optional | List of additional yaml files paths              |

#### Method: `url`
| Name             | Type   | Required    | Description                |
| ---------------- | ------ | ----------- | -------------------------- |
| `manifest_url`   | string | ✅ yes       | URL to remote K8s manifest |
| `namespace`      | string | ✅ yes       | Target namespace           |
| `compose_format` | bool   | 🚫 optional  | Compose format support     |

## 🧮 Computed Outputs
| Name          | Description                     |
| ------------- | ------------------------------- |
| `webhook_id`  | GitOps webhook UUID             |
| `webhook_url` | Full URL to trigger the webhook |

---

## Attributes Reference

| Name | Description                     |
|------|---------------------------------|
| `id` | ID of the created stack         |

