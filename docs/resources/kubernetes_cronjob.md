# ⚙️🧭 **Resource Documentation: `portainer_kubernetes_cronjob`**

# portainer_kubernetes_cronjob

The `portainer_kubernetes_cronjob` resource allows you to deploy one-off Kubernetes `Cronjob` workloads into a specified namespace on a Kubernetes environment (endpoint) managed via Portainer.

---

## Example Usage
### Create Kubernetes Cronjob from YAML
```hcl
resource "portainer_kubernetes_cronjob" "example" {
  endpoint_id = 4
  namespace   = "default"
  manifest    = file("${path.module}/cronjob.yaml")
}
```

## Lifecycle & Behavior
The Cronjob is created via the Portainer Kubernetes API.

Any change results in a delete + create.

To update the Cronjob (e.g. name, image), simply modify the manifest and re-apply:

```sh
terraform apply
```

To remove the Job:
```sh
terraform destroy
```

### Arguments Reference
| Name        | Type   | Required | Description                                                  |
|-------------|--------|----------|--------------------------------------------------------------|
| endpoint_id | int    | ✅ yes   | ID of the Portainer environment (Kubernetes cluster).        |
| namespace   | string | ✅ yes   | Kubernetes namespace where the CronJob should be created.    |
| manifest    | string | ✅ yes   | Kubernetes CronJob manifest (JSON or YAML as a string).      |

---

### Attributes Reference
| Name | Description                               |
|------|-------------------------------------------|
| `id` | 	ID in the format endpoint_id:namespace:cronjob:name    |
