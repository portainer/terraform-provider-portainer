# 🌐 **Resource Documentation: `portainer_webhook_execute`**

# portainer_webhook_execute
The `portainer_webhook_execute` resource allows you to trigger a webhook execution in Portainer – either for restarting a Docker service (via token) or triggering a stack Git update (via stack ID).
> ⚠️ This is an execution resource – it performs an action upon `terraform apply` and doesn't manage state on Portainer.
## Example Usage
### Trigger webhook by token (for Docker service restart/update)
```hcl
resource "portainer_webhook_execute" "restart_service" {
  token = "your-webhook-token"
}
```

### Trigger GitOps webhook for a stack
```hcl
resource "portainer_webhook_execute" "trigger_gitops_stack" {
  stack_id = "your-webhook-token-for-stack"
}
```

### Trigger GitOps webhook for an edge stack
```hcl
resource "portainer_webhook_execute" "trigger_gitops_edge_stack" {
  edge_stack_id = "your-webhook-token-for-edge-stack"
}
```

## Lifecycle & Behavior
- This resource performs a **one-time execution** of a webhook. It supports the following modes:
  - **Docker service restart/update** using a `token`: triggers the `/webhooks/{token}` endpoint.
  - **Stack Git update** using a `stack_id`: triggers the `/stacks/webhooks/{stack_id}` endpoint.
  - **Edge Stack Git update** using an `edge_stack_id`: triggers the `/edge_stacks/webhooks/{edge_stack_id}` endpoint.

- It does **not** track the state of the webhook execution — once applied, it will always recreate the resource if re-applied.

- **Deletion** of this resource is a no-op (it does not delete the actual webhook in Portainer).

> ⚠️ This resource is meant for triggering webhook actions, not managing webhook configurations.

## Arguments Reference

| Name             | Type   | Required    | Description                                                                 |
|------------------|--------|-------------|-----------------------------------------------------------------------------|
| `token`          | string | 🚫 optional | Token used for the `/webhooks/{token}` endpoint (service restart webhook)   |
| `stack_id`       | string | 🚫 optional | Stack ID used for `/stacks/webhooks/{stack_id}` endpoint (GitOps update)    |
| `edge_stack_id`  | string | 🚫 optional | Edge Stack ID used for `/edge_stacks/webhooks/{edge_stack_id}` endpoint     |

> ⚠️ Exactly one of `token`, `stack_id`, or `edge_stack_id` must be set. They are mutually exclusive.

## Attributes Reference
| Name | Description              |
|------|--------------------------|
| `id` | The token or stack ID used to trigger the webhook     |
