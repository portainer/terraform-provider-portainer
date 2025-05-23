# 🧩 **Resource Documentation: `portainer_custom_template`**

# portainer_custom_template

The `portainer_custom_template` resource allows you to manage Portainer Custom Templates.
You can create templates using inline content (`file_content`), from a local file (`file_path`), or from a Git repository.

## Example Usage

### Create Custom Template using `file_content`
```hcl
resource "portainer_custom_template" "example_string" {
  title        = "nginx-string"
  description  = "Template from inline string"
  note         = "This is from string"
  platform     = 1
  type         = 2
  file_content = <<-EOT
    version: "3"
    services:
      web:
        image: nginx
  EOT
}
```

### Create Custom Template from local file
```hcl
resource "portainer_custom_template" "example_file" {
  title        = "nginx-file"
  description  = "Template from file"
  note         = "Note here"
  platform     = 1
  type         = 2
  file_path    = "./templates/nginx.yml"
}
```

### Create Custom Template from Git repository
```hcl
resource "portainer_custom_template" "example_git" {
  title                   = "nginx-git"
  description             = "From Git"
  note                    = "Git-based template"
  platform                = 1
  type                    = 2
  repository_url          = "https://github.com/example/repo"
  repository_username     = "gituser"
  repository_password     = "supersecret"
  repository_reference    = "refs/heads/main"
  compose_file_path       = "docker-compose.yml"
  tlsskip_verify          = false
}
```

## Lifecycle & Behavior
Custom templates are immutable in Portainer. To update, you must destroy and recreate them.
> Only one of `file_content`, `file_path`, or `repository_url` must be specified.
- To delete an custom template created via Terraform, simply run:
```hcl
terraform destroy
```

- To modify a group (e.g., make it dynamic), update the attributes and re-apply:
```hcl
terraform apply
```

## Arguments Reference
### Common Arguments
| Name                | Type         | Required    | Description                                                                |
|---------------------|--------------|-------------|----------------------------------------------------------------------------|
| `title`             | string       | ✅ yes      | Title of the custom template                                               |
| `description`       | string       | ✅ yes      | Description of the template                                                |
| `note`              | string       | ✅ yes      | Display note (can contain HTML)                                            |
| `platform`          | int          | ✅ yes      | Platform: `1` = Linux, `2` = Windows                                       |
| `type`              | int          | ✅ yes      | Stack type: `1` = Swarm, `2` = Compose, `3` = Kubernetes                   |
| `logo`              | string       | 🚫 optional | URL of template logo                                                       |
| `edge_template`     | bool         | 🚫 optional | Whether this is an Edge template (default: `false`)                        |
| `is_compose_format` | bool         | 🚫 optional | Whether to treat as Compose format (default: `false`)                      |
| `variables`         | list(object) | 🚫 optional | List of input variables (`name`, `label`, `description`, `defaultValue`)   |

### For String-based Template
| Name           | Type   | Required    | Description                                                  |
|----------------|--------|-------------|--------------------------------------------------------------|
| `file_content` | string | ✅ required | Inline Compose content (exclusive with `file_path` and `repository_url`) |

### For File-based Template
| Name       | Type   | Required    | Description                                                       |
|------------|--------|-------------|-------------------------------------------------------------------|
| `file_path`| string | ✅ required | Path to local Compose file (exclusive with `file_content` and `repository_url`) |

### For Repository-based Template
| Name                        | Type   | Required    | Description                                                          |
| --------------------------- | ------ | ----------- | -------------------------------------------------------------------- |
| `repository_url`            | string | ✅ required  | Git repository URL                                                  |
| `repository_reference`      | string | ✅ required | Git reference (branch/tag), default: `refs/heads/main`               |
| `compose_file_path`         | string | ✅ required | Path to Compose file inside repo, default: `docker-compose.yml`      |
| `tlsskip_verify`            | bool   | 🚫 optional | Skip TLS verification for Git repo (default: `false`)                |
| `repository_authentication` | bool   | 🚫 optional | Whether to enable authentication for the Git repo (default: `false`) |
| `repository_username`       | string | 🚫 optional | Git username (required if `repository_authentication = true`)        |
| `repository_password`       | string | 🚫 optional | Git password/token (required if `repository_authentication = true`)  |

## Attributes Reference

| Name | Description              |
|------|--------------------------|
| `id` | ID of the custom template in Portainer |
