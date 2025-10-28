# 🌐 Resource Documentation: `portainer_edge_stack_webhook`

## portainer_edge_stack_webhook
The `portainer_edge_stack_webhook` resource allows you to trigger an update of a Portainer-managed edge stack via its webhook.  
This is useful for GitOps workflows or external automation systems.

> `Webhook` currently working only for Portainer BE edition

## Example Usage
```hcl
resource "portainer_edge_stack_webhook" "trigger_my_edge_stack" {
  webhook_id = "65001023-9dd7-415f-9cff-358ba0a78463"  # Webhook token of the edge stack
}
```
## Lifecycle & Behavior
- When you apply this resource, it triggers the execution of the webhook (stack update), simply run:
```hcl
terraform apply
```

## Arguments Reference
| Name          | Type   | Required | Description                                                    |
|---------------|--------|----------|----------------------------------------------------------------|
| `webhook_id`  | string | ✅ yes   | Webhook token for the edge stack to trigger redeployment            |

## Attributes Reference
| Name | Description              |
|------|--------------------------|
| `id` | ID of the webhook trigger|
