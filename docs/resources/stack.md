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
  repository_username       = "gituser"
  repository_password       = "secure"
  repository_reference_name = "refs/heads/main"
  file_path_in_repository   = "docker-compose.yml"
  tlsskip_verify            = false
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

| Name                      | Type          | Required     | Description                                                                |
|---------------------------|---------------|--------------|----------------------------------------------------------------------------|
| `name`                    | string        | ✅ yes       | Name of the stack                                                          |
| `deployment_type`         | string        | ✅ yes       | One of: `standalone`, `swarm`, `kubernetes`                               |
| `method`                  | string        | ✅ yes       | Creation method: `string`, `file`, `repository`, or `url` (K8s only)      |
| `endpoint_id`             | int           | ✅ yes       | ID of the environment where stack will be deployed                        |
| `swarm_id`                | string        | 🚫 optional  | Swarm ID (autofilled if not specified)                                    |
| `namespace`              | string        | 🚫 optional  | Namespace (Kubernetes only)                                               |
| `stack_file_content`      | string        | 🚫 optional  | Inline Compose/YAML content                                               |
| `stack_file_path`         | string        | 🚫 optional  | Path to a Compose file on disk                                            |
| `repository_url`          | string        | 🚫 optional  | Git repository URL                                                        |
| `repository_username`     | string        | 🚫 optional  | Git username                                                              |
| `repository_password`     | string        | 🚫 optional  | Git password/token                                                        |
| `repository_reference_name` | string     | 🚫 optional  | Git reference name (default: `refs/heads/main`)                           |
| `file_path_in_repository` | string        | 🚫 optional  | Path to Compose/K8s manifest inside the repo                              |
| `manifest_url`            | string        | 🚫 optional  | K8s only – URL to remote manifest                                         |
| `compose_format`          | bool          | 🚫 optional  | Use Compose format for K8s (default: `false`)                             |
| `env`                     | list(object)  | 🚫 optional  | List of env variables (`name`, `value`)                                   |
| `tlsskip_verify`          | bool          | 🚫 optional  | Skip TLS verification for Git repository (default: `false`)               |

---

## Attributes Reference

| Name | Description                     |
|------|---------------------------------|
| `id` | ID of the created stack         |
