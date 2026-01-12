# 🪝 **Data Source Documentation: `portainer_webhook`**

# portainer_webhook
The `portainer_webhook` data source allows you to look up an existing Portainer webhook by its resource ID and endpoint ID. This is useful for retrieving the secret token of a webhook for automation purposes.

## Example Usage

### Look up a webhook by resource and endpoint

```hcl
data "portainer_webhook" "stack_webhook" {
  resource_id = "12"        # ID of the stack or service
  endpoint_id = 1           # ID of the environment
}

output "webhook_url" {
  value = "https://portainer.example.com/api/webhooks/${data.portainer_webhook.stack_webhook.token}"
}
```

## Arguments Reference

| Name          | Type    | Required | Description                                     |
|---------------|---------|----------|-------------------------------------------------|
| `resource_id` | string  | ✅ yes   | ID of the resource associated with the webhook. |
| `endpoint_id` | integer | ✅ yes   | ID of the environment.                          |

## Attributes Reference

| Name           | Type    | Description                               |
|----------------|---------|-------------------------------------------|
| `id`           | string  | ID of the Portainer webhook.             |
| `token`        | string  | Secret token for triggering the webhook. |
| `webhook_type` | integer | Type of webhook.                          |
