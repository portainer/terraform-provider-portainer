---
page_title: "portainer_stack Resource - Portainer"
subcategory: ""
description: |-
  The portainer_stack resource allows you to manage application stacks in Portainer.
---

# portainer_stack (Resource)

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
  sensitive   = true
}
```

### Deploy Standalone Stack from Git Repository (Ephemeral Credentials)
```hcl
resource "portainer_stack" "standalone_repo_ephemeral" {
  name                      = "your-standalone-repo"
  deployment_type           = "standalone"
  method                    = "repository"
  endpoint_id               = 1

  # Ephemeral (write-only) Git credentials
  repository_url_wo         = "https://github.com/example/private-repo"
  repository_username_wo    = "gituser"
  repository_password_wo    = "super-secret-token"
  repository_wo_version     = 1

  repository_reference_name = "refs/heads/main"
  file_path_in_repository   = "docker-compose.yml"
  tlsskip_verify            = false
  additional_files = [
    "some-other-docker-compose.yml"
  ]

  stack_webhook             = true
  update_interval           = "10m"
  pull_image                = true
  force_update              = true
  git_repository_authentication = true

  env {
    name  = "ENV"
    value = "production"
  }
}
```

### Deploy Stack with Write-only (Ephemeral) Environment Variables
Values in `env_wo` are sent to Portainer but never stored in the Terraform state or plan, so they can come from ephemeral resources (e.g. `ephemeral "vault_kv_secret_v2"`). Requires Terraform 1.11+ or OpenTofu 1.11+.
```hcl
ephemeral "vault_kv_secret_v2" "db" {
  mount = "secret"
  name  = "app/db"
}

resource "portainer_stack" "app" {
  name               = "app"
  deployment_type    = "standalone"
  method             = "string"
  endpoint_id        = 1
  stack_file_content = file("docker-compose.yml")

  env {
    name  = "TZ"
    value = "Europe/Prague"
  }

  env_wo {
    name  = "DB_PASSWORD"
    value = ephemeral.vault_kv_secret_v2.db.data["password"]
  }

  # Write-only values cannot be compared: bump this to redeploy with new values.
  env_wo_version = 1
}
```

- The `env_wo` values are sent on every deploy together with `env`, because Portainer replaces the whole variable list on each deploy.
- Only the names of `env_wo` variables are stored in state; Read keeps them out of `env`. After `terraform import`, all variables land in `env` until `env_wo` is configured and applied.
- A name may appear only once across `env` and `env_wo`.
- The values are still stored in Portainer's database and are visible to users who can see the stack in Portainer.

### Deploy with Specific Registries
```hcl
resource "portainer_stack" "app_with_registries" {
  name               = "app-with-custom-registries"
  deployment_type    = "standalone"
  method             = "string"
  endpoint_id        = 1
  stack_file_content = file("./docker-compose.yml")

  # List of Portainer registry IDs allowed for this stack
  registries = [12, 15]
}
```

### Deploy Stack with Access Control
```hcl
resource "portainer_stack" "restricted_stack" {
  name             = "restricted-stack"
  deployment_type  = "standalone"
  method           = "string"
  endpoint_id      = 1
  
  stack_file_content = <<-EOT
    version: "3"
    services:
      web:
        image: nginx
  EOT

  # Access Control
  ownership        = "restricted"
  authorized_teams = [1, 2] # IDs of authorized teams
  authorized_users = [5]    # IDs of authorized users
}
```

### Deploy from Git Repository using existing Credentials
```hcl
resource "portainer_stack" "repo_with_creds" {
  name                      = "repo-with-existing-creds"
  deployment_type           = "standalone"
  method                    = "repository"
  endpoint_id               = 1
  repository_url            = "https://github.com/example/private-repo"
  repository_reference_name = "refs/heads/main"
  file_path_in_repository   = "docker-compose.yml"

  # Reference existing Git credentials by ID instead of providing username/password
  # Portainer < 2.43: shared Git credential ID
  repository_git_credential_id = 5

  # Portainer 2.43 STS+: the credentials model was replaced by "Sources".
  # Reference an existing Source instead; on 2.43+ repository_git_credential_id is ignored.
  # source_id = 3
}
```

**Portainer 2.43+ STS compatibility**
- In 2.43 the shared Git credentials were replaced by the **Sources** model, and the stack API dropped `repositoryGitCredentialID` in favor of `SourceID`.
- The provider sends both fields, so a single configuration works against old and new Portainer: set `repository_git_credential_id` for Portainer < 2.43 and `source_id` for 2.43+. On 2.43+, if you keep only `repository_git_credential_id` together with `git_repository_authentication = true` and no password, Portainer rejects the deploy with *"Password must be specified when authentication is enabled"* — set `source_id` (or fall back to inline `repository_username`/`repository_password`) instead.

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
  additional_files = [
    "some-other-docker-compose.yml"
  ]
}
```
### Deploy Swarm Stack from Git Repository (Ephemeral Credentials)
```hcl
resource "portainer_stack" "swarm_repo_ephemeral" {
  name                      = "your-swarm-repo"
  deployment_type           = "swarm"
  method                    = "repository"
  endpoint_id               = 1

  # Ephemeral (write-only) credentials
  repository_url_wo         = "https://github.com/example/private-repo"
  repository_username_wo    = "gituser"
  repository_password_wo    = "super-secret-token"
  repository_wo_version     = 1   # bump this value to trigger rotation

  repository_reference_name = "refs/heads/main"
  file_path_in_repository   = "docker-compose.yml"

  env {
    name  = "ENV"
    value = "production"
  }
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
### Deploy Kubernetes Stack from Repository (Ephemeral Credentials)
```hcl
resource "portainer_stack" "k8s_repo_ephemeral" {
  name                      = "kube-stack"
  deployment_type           = "kubernetes"
  method                    = "repository"
  endpoint_id               = 2

  # Ephemeral (write-only) Git credentials
  repository_url_wo         = "https://github.com/example/private-repo"
  repository_username_wo    = "gituser"
  repository_password_wo    = "super-secret-token"
  repository_wo_version     = 1

  repository_reference_name = "refs/heads/main"
  file_path_in_repository   = "kube.yaml"
  namespace                 = "default"
  compose_format            = true
  additional_files = [
    "some-other-file.yaml"
  ]
}
```

### Deploy Kubernetes Stack from Repository (Helm Chart)
```hcl
resource "portainer_stack" "k8s_helm_repo" {
  name                      = "kube-helm-stack"
  deployment_type           = "kubernetes"
  method                    = "repository"
  endpoint_id               = 2
  repository_url            = "https://github.com/example/repo"
  repository_reference_name = "refs/heads/main"
  namespace                 = "default"
  helm_chart_path           = "charts/my-chart"

  # Optional: environment-specific Helm values overrides
  additional_helm_values_files = [
    "values-test.yaml",
    "values-prod.yaml"
  ]

  # Optional, Portainer BE 3.1+: roll back a failed Helm deployment
  atomic = true
}
```
> **Note:** When `helm_chart_path` is set, `file_path_in_repository` is not required.

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
### Stop/Start a Stack
```hcl
resource "portainer_stack" "my_stack" {
  name            = "my-stack"
  deployment_type = "swarm"
  method          = "string"
  endpoint_id     = 1

  stack_file_content = <<-EOT
    version: "3"
    services:
      web:
        image: nginx
  EOT

  # Set to false to stop the stack, true to start it
  active = false
}
```

---

## Timeouts

This resource supports the following [timeouts](https://developer.hashicorp.com/terraform/language/resources/syntax#operation-timeouts) configuration options:

| Operation | Default  | Description                          |
|-----------|----------|--------------------------------------|
| `create`  | 30m      | Time to wait for stack creation      |
| `update`  | 30m      | Time to wait for stack update        |
| `delete`  | 10m      | Time to wait for stack deletion      |

If the delete operation encounters a server error (5xx), it will automatically retry every 15 seconds until the timeout is reached.

### Example
```hcl
resource "portainer_stack" "example" {
  name            = "my-stack"
  deployment_type = "standalone"
  method          = "string"
  endpoint_id     = 1

  stack_file_content = <<-EOT
    version: "3"
    services:
      web:
        image: nginx
  EOT

  timeouts {
    create = "15m"
    update = "15m"
    delete = "3m"
  }
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

### Asynchronous deployment on Portainer 2.45+

From Portainer 2.45 a stack deploy is asynchronous: the create call returns as soon as the deploy job is queued, and Portainer keeps the stack in status `Deploying` until `docker compose up` actually finishes — rejecting every mutating call in that window with `409 Stack deployment is already in progress`.

The provider waits for the deployment to settle before it sends anything else to that stack, so no configuration change is needed. Two consequences worth knowing:

- **A create can take as long as the deployment does.** Image pulls are the usual reason. The wait is bounded by the resource's `create`/`update` timeout (30 minutes by default), so raise that rather than expecting the apply to return early.
- **A deployment that fails is reported as a failure.** Portainer releases the lock either way, so the provider checks the resulting status instead of assuming success: a stack that ends in `Error` fails the apply with Portainer's error message (or a pointer to the stack's logs), rather than reporting a healthy apply for a stack that never came up. This covers creates and, through the deployment history Portainer keeps since 2.45, updates and `portainer_deploy` redeploys as well.

On Portainer 2.39 and other pre-2.45 versions deployment is synchronous and no lock exists, so nothing changes.

#### Recovering a stack left in `Error`

Portainer does not clear the `Error` status by itself - it stays until something deploys the stack again — and it only persists a new compose file once a deploy has succeeded. A stack whose deployment failed therefore sits in `Error` with its previous definition still on disk.

The check above applies only to a deployment this provider started: an update is blamed only for a deployment that started with or after it (Portainer 2.45+ keeps that history; an older version is never blamed for an earlier failure). Before an update, a start or a stop, the provider waits out a deployment that is still running but does **not** treat an earlier failure as fatal: that call is exactly what replaces the broken definition. Fixing the compose file and re-applying recovers the stack, with no manual step in the Portainer UI.

Note that a stack can be reported as failed for reasons that have nothing to do with your configuration - see [portainer/portainer#13281](https://github.com/portainer/portainer/issues/13281), where a historical failed Swarm task makes a successful update report `deployment failed`. Being able to re-apply matters more in that case, not less.

---

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `deployment_type` (String) Deployment mode: 'standalone', 'swarm', or 'kubernetes'
- `endpoint_id` (Number) Identifier of the Portainer environment (endpoint) where the stack will be deployed. Changing this value forces resource recreation.
- `method` (String) Creation method: 'string', 'file', 'repository', or 'url'
- `name` (String) Name of the Portainer stack. Must be unique within the target endpoint. Changing this value forces resource recreation.

### Optional

> **NOTE**: [Write-only arguments](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments) are supported in Terraform 1.11 and later.

- `active` (Boolean) Whether the stack should be running. Set to false to stop the stack.
- `additional_files` (List of String) List of additional Compose file paths to use when deploying from Git repository.
- `additional_helm_values_files` (List of String) List of additional Helm values files (e.g. values-prod.yaml). Only used with helm_chart_path.
- `atomic` (Boolean) Helm chart stacks only (deployment_type 'kubernetes', method 'repository' with helm_chart_path), Portainer Business Edition 3.1 or later: whether a failed Helm deployment is rolled back automatically (Helm --rollback-on-failure). Leaving it out sends nothing and keeps the choice Portainer has recorded; it is read back only while configured. Changed in place.
- `authorized_teams` (Set of Number) List of team IDs authorized to access this stack (only if ownership is restricted).
- `authorized_users` (Set of Number) List of user IDs authorized to access this stack (only if ownership is restricted).
- `compose_format` (Boolean) Whether the supplied content is in Docker Compose format (true) instead of native Kubernetes manifest (false). Only applies to Kubernetes stacks. Changing this value forces resource recreation.
- `env` (Block List) List of environment variables injected into the stack at deploy time. (see [below for nested schema](#nestedblock--env))
- `env_wo` (Block List) Environment variables whose values are write-only (supports ephemeral values; the values are not stored in Terraform state, the names are). Sent together with env on every deploy. Write-only values cannot be compared, so increment env_wo_version to redeploy the stack with changed values. (see [below for nested schema](#nestedblock--env_wo))
- `env_wo_version` (Number) Version flag for env_wo; change it to redeploy the stack with the current env_wo values.
- `file_path_in_repository` (String) Path to Compose/manifest file in the repository. Defaults to docker-compose.yml for Docker/Swarm stacks. Not required when helm_chart_path is set.
- `filesystem_path` (String) Local filesystem path on the host used when support_relative_path is true. Maps to the repository working directory.
- `force_update` (Boolean) Whether to prune unused services/networks during stack update (default: true)
- `git_repository_authentication` (Boolean) Whether the Git repository requires authentication. When true, repository_username and repository_password (or their write-only equivalents) are sent to Portainer.
- `helm_chart_path` (String) Path to a Helm chart folder in the Git repository (must contain Chart.yaml). Only used when deployment_type is 'kubernetes' and method is 'repository'.
- `kubernetes_force_apply` (Boolean) Kubernetes stacks only, Portainer 3.0 or later: whether a deployment deletes and recreates a Kubernetes resource whose immutable fields changed instead of failing on it. Recreating destroys the resource and everything tied to its identity. Portainer leaves it off when this is left out; leaving it out keeps the choice Portainer has recorded and sends nothing. Changed in place for method 'repository'; for the other methods a change replaces the stack.
- `kubernetes_prune` (Boolean) Kubernetes stacks only, Portainer 3.0 or later: whether a deployment deletes the Kubernetes resources an earlier deployment applied that the manifest no longer declares. Portainer prunes when this is left out; leaving it out keeps the choice Portainer has recorded and sends nothing, so older Portainer versions are unaffected. Changed in place for method 'repository'; for the other methods Portainer can only set it on create, so a change replaces the stack.
- `manifest_url` (String) URL to a remote Kubernetes manifest used when deployment_type is 'kubernetes' and method is 'url'. Changing this value forces resource recreation.
- `namespace` (String) Kubernetes namespace used when deployment_type is 'kubernetes'. Changing this value forces resource recreation.
- `ownership` (String) Ownership level: 'public', 'administrators' or 'restricted'.
- `prune` (Boolean) Whether to prune unused services/networks during stack update (default: false)
- `pull_image` (Boolean) Whether to force pull latest images during stack update (default: true)
- `registries` (List of Number) List of registry IDs allowed for this stack.
- `repository_git_credential_id` (Number) ID of the shared Git credentials to use for authentication (Portainer < 2.43). Replaced by 'source_id' in Portainer 2.43 STS, which switched to the Sources model; on 2.43+ this field is ignored. Both are sent, so a single configuration stays compatible with old and new Portainer.
- `repository_password` (String, Sensitive) Password or personal access token used to authenticate against the Git repository. Stored in state as a sensitive value.
- `repository_password_wo` (String, Sensitive, [Write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments)) Write-only repository password (supports ephemeral values; not stored in Terraform state).
- `repository_reference_name` (String) Git reference (branch or tag) used by Portainer when deploying from the repository. Defaults to refs/heads/main.
- `repository_url` (String) URL of the Git repository used to deploy the stack when method is 'repository'. Changing this value forces resource recreation.
- `repository_url_wo` (String, [Write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments)) Write-only repository URL (supports ephemeral values; not stored in Terraform state).
- `repository_username` (String) Username used to authenticate against the Git repository.
- `repository_username_wo` (String, Sensitive, [Write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments)) Write-only repository username (supports ephemeral values).
- `repository_wo_version` (Number) Version flag for write-only repository credentials; increment to trigger recreation.
- `source_id` (Number) ID of a Portainer Source (Git source) providing the repository URL and credentials. Introduced in Portainer 2.43 STS, which replaced 'repository_git_credential_id'. When set (non-zero), Portainer resolves the repository URL/credentials from the referenced Source and ignores the inline repository_username/repository_password and repository_git_credential_id fields.
- `stack_file_content` (String) Inline Compose or Kubernetes manifest content used to deploy the stack. Required when method is 'string'; populated from stack_file_path when method is 'file'.
- `stack_file_path` (String) Local filesystem path to a Compose or manifest file. Contents are read and uploaded to Portainer when method is 'file'.
- `stack_webhook` (Boolean) Enable autoUpdate webhook (GitOps).
- `support_relative_path` (Boolean) Whether Portainer should support relative paths inside the Compose file for bind mounts referencing repository contents. Changing this value forces resource recreation.
- `swarm_id` (String) Identifier of the Docker Swarm cluster used when deployment_type is 'swarm'. Automatically fetched from Portainer when not provided. Changing this value forces resource recreation.
- `timeouts` (Block, Optional) (see [below for nested schema](#nestedblock--timeouts))
- `tlsskip_verify` (Boolean) Whether to skip TLS verification when Portainer connects to the Git repository. Changing this value forces resource recreation.
- `update_interval` (String) GitOps auto-update polling interval (e.g. '5m', '1h'). When set, Portainer periodically checks the Git repository for changes and redeploys the stack.

### Read-Only

- `id` (String) Identifier of the stack.
- `resource_control_id` (Number) Identifier of the Portainer resource control entry associated with the stack. Computed by Portainer.
- `webhook_id` (String) UUID of the GitOps webhook (read-only).
- `webhook_url` (String, Sensitive) Full URL of the webhook trigger. Sensitive: anyone with the URL can redeploy the stack.

<a id="nestedblock--env"></a>
### Nested Schema for `env`

Required:

- `name` (String) Name of the environment variable.
- `value` (String) Value of the environment variable.


<a id="nestedblock--env_wo"></a>
### Nested Schema for `env_wo`

Required:

> **NOTE**: [Write-only arguments](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments) are supported in Terraform 1.11 and later.

- `name` (String) Name of the environment variable.
- `value` (String, Sensitive, [Write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments)) Write-only value of the environment variable.


<a id="nestedblock--timeouts"></a>
### Nested Schema for `timeouts`

Optional:

- `create` (String) A string that can be [parsed as a duration](https://pkg.go.dev/time#ParseDuration) consisting of numbers and unit suffixes, such as "30s" or "2h45m". Valid time units are "s" (seconds), "m" (minutes), "h" (hours).
- `delete` (String) A string that can be [parsed as a duration](https://pkg.go.dev/time#ParseDuration) consisting of numbers and unit suffixes, such as "30s" or "2h45m". Valid time units are "s" (seconds), "m" (minutes), "h" (hours). Setting a timeout for a Delete operation is only applicable if changes are saved into state before the destroy operation occurs.
- `update` (String) A string that can be [parsed as a duration](https://pkg.go.dev/time#ParseDuration) consisting of numbers and unit suffixes, such as "30s" or "2h45m". Valid time units are "s" (seconds), "m" (minutes), "h" (hours).

## Notes on Arguments and Attributes

- `env`: List of environment variables (`name`, `value`)
- `pull_image`: When `true`, updates and git redeploys also send `RepullImageAndRedeploy`, which replaces the `pullImage` flag Portainer deprecated in 2.36.
- `active`: Defaults to `true`.
- `file_path_in_repository`: Changed in place on Portainer 2.42+. Older versions report an error and the stack has to be replaced.
- `git_repository_authentication`: Defaults to `false`.
- `repository_wo_version`: Must be set when using `_wo` fields.
- `stack_webhook`: Defaults to `false`.
- `update_interval`: Polling interval (enables GitOps polling, e.g. `30m`, `1h`)
- `force_update`: Whether to force redeploy (default: `false`)
- `support_relative_path`: Defaults to `false`.
- `compose_format`: Defaults to `false`.
- `tlsskip_verify`: Defaults to `false`.
- `helm_chart_path`: When set, `file_path_in_repository` is not required.
- `atomic`: Helm chart stacks only (Portainer BE 3.1+). Sent on create and on every Git settings update only when configured; removing it leaves Portainer's recorded choice alone. Portainer before 3.1 ignores it on create, which then shows as a difference on the next plan.

## Arguments by Deployment Method

### 🐳 Docker Stack (standalone/swarm)
#### Method: `string`
Arguments: `stack_file_content`.

#### Method: `file`
Arguments: `stack_file_path`.

#### Method: `repository`
Arguments: `repository_url`, `repository_reference_name`, `file_path_in_repository`, `tlsskip_verify`, `git_repository_authentication`, `repository_username`, `repository_password`, `repository_url_wo`, `repository_username_wo`, `repository_password_wo`, `repository_wo_version`, `stack_webhook`, `update_interval`, `force_update`, `support_relative_path`, `filesystem_path`, `additional_files`, `repository_git_credential_id`, `source_id`.

#### Extra for `swarm`
Arguments: `swarm_id`.

---

### ☸️ Kubernetes Stack
#### Method: `string`
Arguments: `stack_file_content`, `namespace`, `compose_format`, `kubernetes_prune`, `kubernetes_force_apply`.

#### Method: `repository`
Arguments: `repository_url`, `namespace`, `repository_reference_name`, `file_path_in_repository`, `tlsskip_verify`, `git_repository_authentication`, `repository_username`, `repository_password`, `repository_url_wo`, `repository_username_wo`, `repository_password_wo`, `repository_wo_version`, `stack_webhook`, `update_interval`, `force_update`, `compose_format`, `kubernetes_prune`, `kubernetes_force_apply`, `additional_files`, `helm_chart_path`, `additional_helm_values_files`, `atomic`, `repository_git_credential_id`, `source_id`.

#### Method: `url`
Arguments: `manifest_url`, `namespace`, `compose_format`, `kubernetes_prune`, `kubernetes_force_apply`.

## Import

Stacks can be imported as `<endpoint_id>:<stack_id>:<deployment_type>[:<method>]`. The resource ID is the stack ID alone. `method` may be left out for a stack deployed from Git, which is imported with `method = "repository"`; any other stack needs it (`string`, `file` or `url`), since Portainer does not record which of the three created it. The format from before v3.0.0, `<endpoint_id>-<stack_id>-<deployment_type>[-<method>]`, is still accepted.

```shell
terraform import portainer_stack.example 1:5:standalone:string
```

### Import by identity

Terraform 1.12 and later can also import by resource identity: `endpoint_id`, the stack `id` and its `deployment_type` in an `import` block. The ID string above keeps working.

```hcl
import {
  to       = portainer_stack.example
  identity = {
    endpoint_id     = 1
    id              = "5"
    deployment_type = "standalone"
  }
}
```

The identity has no `method`, since Portainer does not record it. An import by identity (and a [`portainer_stack` list resource](../list-resources/stack.md) result with `include_resource = true`) infers it: `repository` for a stack deployed from Git, `string` for any other, with `stack_file_content` read from Portainer's copy of the stack file. A stack configured with `method = "string"` or `"repository"` therefore plans no change after the import. For a stack configured with `method = "file"` or `"url"`, import by ID with the method (`1:5:standalone:file`) instead: imported by identity, its method would differ from the configuration and the plan would replace the stack.

`terraform query` (Terraform 1.14+) with the [`portainer_stack` list resource](../list-resources/stack.md) finds the objects to import and can generate the `import` blocks (`terraform query -generate-config-out=generated.tf`). The identity is stored in the state from v3.1.0 on; state written by v3.0.0 gets it on the next refresh, without a plan change. OpenTofu supports neither resource identity nor list resources; it keeps importing by ID.
