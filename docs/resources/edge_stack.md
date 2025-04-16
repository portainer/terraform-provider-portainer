# 🧩 **Resource Documentation: `portainer_custom_template`**

# portainer_custom_template
The `portainer_custom_template` resource allows you to manage Portainer Custom Templates.
You can create templates using inline content (`file_content`), from a local file (`file_path`), or from a Git repository.

## Example Usage
### Create Custom Template from `file_content`
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

### Create Custom Template from file
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
> ⚠️ One of `file_content`, `file_path`, or `repository_url` **must** be provided.

---

## Arguments Reference

| Name                    | Type          | Required    | Description                                                                |
|-------------------------|---------------|-------------|----------------------------------------------------------------------------|
| `title`                 | string        | ✅ yes      | Title of the custom template                                               |
| `description`           | string        | ✅ yes      | Description of the template                                                |
| `note`                  | string        | ✅ yes      | Display note (can contain HTML)                                            |
| `platform`              | int           | ✅ yes      | Platform: `1` = Linux, `2` = Windows                                       |
| `type`                  | int           | ✅ yes      | Stack type: `1` = Swarm, `2` = Compose, `3` = Kubernetes                   |
| `file_content`          | string        | 🚫 optional | Inline Compose content                                                     |
| `file_path`             | string        | 🚫 optional | Path to Compose file                                                       |
| `repository_url`        | string        | 🚫 optional | Git repository URL                                                         |
| `repository_username`   | string        | 🚫 optional | Git username                                                               |
| `repository_password`   | string        | 🚫 optional | Git password/token                                                         |
| `repository_reference`  | string        | 🚫 optional | Git reference (branch/tag), default: `refs/heads/main`                     |
| `compose_file_path`     | string        | 🚫 optional | Path to Compose file inside the repo, default: `docker-compose.yml`       |
| `tlsskip_verify`        | bool          | 🚫 optional | Skip TLS verification for Git repo (default: `false`)                     |
| `logo`                  | string        | 🚫 optional | URL of template logo                                                       |
| `edge_template`         | bool          | 🚫 optional | Whether this is an Edge template (default: `false`)                        |
| `is_compose_format`     | bool          | 🚫 optional | If Compose format (default: `false`)                                       |
| `variables`             | list(object)  | 🚫 optional | List of input variables (`name`, `label`, `description`, `defaultValue`)  |

---

## Attributes Reference

| Name | Description                     |
|------|---------------------------------|
| `id` | ID of the Custom Template in Portainer |
