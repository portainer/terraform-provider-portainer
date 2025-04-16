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

### Trigger webhook by token (for Docker service restart/update)
```hcl
resource "portainer_webhook_execute" "restart_service" {
  token = "your-webhook-token"
}
```

## Lifecycle & Behavior
- This resource performs a **one-time execution** of a webhook, either:
  - Restarting a Docker service via `/webhooks/{token}` using the `token`, or
  - Triggering a Git update for a stack via `/stacks/webhooks/{stack_id}` using the `stack_id`.

- It does **not** track the state of the webhook execution — once applied, it will always recreate the resource if re-applied.

- **Deletion** of this resource is a no-op (it does not delete the actual webhook in Portainer).

> ⚠️ This resource is meant for triggering webhook actions, not managing webhook configurations.

## Arguments Reference
| Name       | Type   | Required | Description                                                                 |
|------------|--------|----------|-----------------------------------------------------------------------------|
| `token`    | string | 🚫 optional | Token used for the `/webhooks/{token}` endpoint (service restart)           |
| `stack_id` | string | 🚫 optional | Stack ID used for `/stacks/webhooks/{stack_id}` endpoint (git update)       |

> ✅ You must provide **either** `token` or `stack_id`, but not both.

## Attributes Reference
| Name | Description              |
|------|--------------------------|
| `id` | The token or stack ID used to trigger the webhook     |
