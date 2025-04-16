# 🧭 **Resource Documentation: `portainer_kubernetes_job`**

# portainer_kubernetes_job

The `portainer_kubernetes_job` resource allows you to deploy one-off Kubernetes `Job` workloads into a specified namespace on a Kubernetes environment (endpoint) managed via Portainer.

---

## Example Usage
### Create Kubernetes Job from YAML

```hcl
resource "portainer_kubernetes_job" "example" {
  endpoint_id = 4
  namespace   = "default"
  manifest    = file("${path.module}/job.yaml")
}
```

## Lifecycle & Behavior
The Job is created via the Portainer Kubernetes API.

Any change results in a delete + create.

To update the Job (e.g. name, image), simply modify the manifest and re-apply:

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
| namespace   | string | ✅ yes   | Kubernetes namespace where the Job should be created.        |
| manifest    | string | ✅ yes   | Kubernetes Job manifest (JSON or YAML as a string).          |

---

### Attributes Reference
| Name | Description                               |
|------|-------------------------------------------|
| `id` | 	ID in the format endpoint_id:namespace:job:name    |
