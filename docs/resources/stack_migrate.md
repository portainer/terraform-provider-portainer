# Resource Documentation: `portainer_stack_migrate`

# portainer_stack_migrate
The `portainer_stack_migrate` resource triggers migration of a stack from one Portainer environment (endpoint) to another. It re-creates the stack in the target environment before removing the original.

> Note: This is an action resource. It performs the migration on `terraform apply` and does not track ongoing state. Each apply triggers a new migration.

## Example Usage

```hcl
resource "portainer_stack_migrate" "move" {
  stack_id           = 5
  target_endpoint_id = 2
}
```

### Migrate with rename

```hcl
resource "portainer_stack_migrate" "move_renamed" {
  stack_id           = 5
  target_endpoint_id = 2
  stack_name         = "my-stack-production"
}
```

### Migrate to a Swarm environment

```hcl
resource "portainer_stack_migrate" "move_swarm" {
  stack_id           = 5
  target_endpoint_id = 3
  swarm_id           = "jpofkc0i9uo9wtx1zesuk649w"
}
```

## Arguments Reference

| Name                 | Type   | Required | Description                                                                                  |
|----------------------|--------|----------|----------------------------------------------------------------------------------------------|
| `stack_id`           | number | Yes      | Stack identifier to migrate.                                                                 |
| `target_endpoint_id` | number | Yes     | Target environment (endpoint) identifier.                                                    |
| `stack_name`         | string | No       | New name for the stack after migration. If not set, the original name is kept.               |
| `swarm_id`           | string | No       | Swarm cluster identifier (required when migrating to a Swarm environment).                   |
| `endpoint_id`        | number | No       | Source environment identifier. Required for stacks created before Portainer 1.18.0.          |
