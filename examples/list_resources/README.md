# List resources and import by identity

Finds existing Portainer objects with `terraform query` and imports them by
resource identity.

- `portainer.tfquery.hcl` lists tags, environments, users, teams, environment
  groups, Edge groups, registries, custom templates and webhooks, plus the
  Docker networks, volumes and secrets and the stacks of one environment
  (`docker_endpoint_id`).
- It also lists Kubernetes Deployments, ConfigMaps and ClusterRoles of one
  Kubernetes environment (`kubernetes_endpoint_id`) and the Edge stacks and
  jobs.
- `main.tf` has a commented `import` block that imports a tag by identity.

## Requirements

- Terraform 1.14 or later for `terraform query` and list resources; import by
  identity alone needs Terraform 1.12. OpenTofu supports neither.
- terraform-provider-portainer 3.1.0 or later.

## Usage

```shell
terraform init
terraform query -var portainer_url=https://portainer.example.com -var portainer_api_key=...

# Write resource and import blocks for every result, then review and apply them:
terraform query -generate-config-out=generated.tf
terraform plan
```

The list resources are documented in [docs/list-resources](../../docs/list-resources/).
