# 🔁 **Resource Documentation: `portainer_endpoint_service_update`**

# portainer_endpoint_service_update
The `portainer_endpoint_service_update` resource allows you to force an update of a Docker service on a specified endpoint in Portainer. It can optionally pull the latest image before updating the service.

## Example Usage
```hcl
resource "portainer_endpoint_service_update" "force_update_some_service" {
  endpoint_id   = 1
  service_name  = "my-name_service"
  pull_image    = true
}
```

## Lifecycle & Behavior
- This resource triggers a one-time force update of a Docker service on the given endpoint.
- The service is located based on its service_name.
- If pull_image is set to true, Portainer will pull the latest image before updating.
Update service run by:
```hcl
trraform apply
```
> Note: This resource does not persist – it's meant for imperative actions like force-pulling & restarting a service.

## Arguments Reference
| Name           | Type   | Required | Description                                                                 |
|----------------|--------|----------|-----------------------------------------------------------------------------|
| `endpoint_id`  | number | ✅ yes   | ID of the Portainer endpoint                                                |
| `service_name` | string | ✅ yes   | Name of the Docker service to update (must exist on the endpoint)          |
| `pull_image`   | bool   | 🚫 no    | Whether to pull the latest image before updating the service (default: false) |

## Attributes Reference
| Name | Description                                            |
|------|--------------------------------------------------------|
| `id` | ID of the resource (`endpointID-serviceName`)          |
