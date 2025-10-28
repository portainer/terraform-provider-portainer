# 🌐 **Resource Documentation: `portainer_webhook`**

# portainer_webhook
The `portainer_webhook` resource allows you to create and manage webhooks in Portainer. Webhooks can be used to trigger actions such as pulling and redeploying stacks or images via external systems.

> Currently working only for Portainer BE edition

## Example Usage
```hcl
resource "portainer_webhook" "stack" {
  endpoint_id   = 1
  resource_id   = "3"        # Stack ID
  webhook_type  = 1          # 1 = Stack
}
```
## Lifecycle & Behavior
- Create webhook, simply run:
```hcl
terraform destroy
```

- Delete webhook:
```hcl
terraform apply
```

## Arguments Reference
| Name           | Type   | Required | Description                                                                                                   |
| -------------- | ------ | -------- | ------------------------------------------------------------------------------------------------------------- |
| `endpoint_id`  | number | ✅ yes    | ID of the Portainer environment (endpoint).                                                                  |
| `resource_id`  | string | ✅ yes    | ID of the target resource (Stack or Registry).                                                               |
| `registry_id`  | number | 🚫 optional | ID of the registry (optional, used when webhook is linked to a registry).                                  |
| `webhook_type` | number | ✅ yes    | Type of the webhook:<br>• `1` = Stack<br>• *(reserved for future use)* other values for Registries or Images. |

## Attributes Reference
| Name | Description              |
|------|--------------------------|
| `id` | ID of the created webhook in Portainer     |
| `token` |	Webhook token (used to trigger the webhook) |
