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
| `reassign_on_change` | bool | 🚫 no | Reassign the existing webhook instead of replacing it when `resource_id` or `webhook_type` changes, keeping its token. **Business Edition only.** Defaults to `false`. |

## Attributes Reference
| Name | Description              |
|------|--------------------------|
| `id` | ID of the created webhook in Portainer     |
| `token` |	Webhook token (used to trigger the webhook) |

## Repointing a webhook

By default, changing `resource_id` or `webhook_type` **replaces** the webhook. The replacement gets a new token, so every URL already handed out stops working.

Portainer Business Edition can move a webhook to another resource in place, which keeps its token. Set `reassign_on_change = true` to use it:

```hcl
resource "portainer_webhook" "deploy" {
  endpoint_id        = portainer_environment.prod.id
  resource_id        = portainer_stack.web.id
  webhook_type       = 1
  reassign_on_change = true
}
```

It is off by default on purpose: Portainer CE has no reassign endpoint, so turning it on there makes the apply fail rather than replace the webhook the way it always has.
