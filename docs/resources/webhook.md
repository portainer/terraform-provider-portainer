# 🌐 **Resource Documentation: `portainer_webhook`**

# portainer_webhook
The `portainer_webhook` resource allows you to create and manage webhooks in Portainer. Webhooks can be used to trigger actions such as pulling and redeploying stacks or images via external systems.

## Example Usage
```hcl
resource "portainer_webhook" "stack" {
  endpoint_id   = 1
  resource_id   = "3"        # Stack ID
  webhook_type  = 0          # 0 = Stack
}
```
## Lifecycle & Behavior
Team membrship are updated if any of the attributes change (e.g. role).

- To delete a membrship created via Terraform, simply run:
```hcl
terraform destroy
```

- To change a team membrship role id, update the role field and re-apply:
```hcl
terraform apply
```

## Arguments Reference
| Name          | Type   | Required | Description                                                    |
|---------------|--------|----------|----------------------------------------------------------------|
| `endpoint_id` | number | ✅ yes   | ID of the Portainer environment (endpoint)                     |
| `resource_id` | string | ✅ yes   | ID of the resource (Stack or Registry)                         |
| `webhook_type`| number | ✅ yes   | Type of the webhook:<br>`1` = Stack         |

## Attributes Reference
| Name | Description              |
|------|--------------------------|
| `id` | ID of the created webhook in Portainer     |
| `token` |	Webhook token (used to trigger the webhook) |
