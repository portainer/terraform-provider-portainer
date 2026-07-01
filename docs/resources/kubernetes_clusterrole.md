# 🌐👤 **Resource Documentation: `portainer_kubernetes_clusterrole`**

# portainer_kubernetes_clusterrole

The `portainer_kubernetes_clusterrole` resource allows you to deploy one-off Kubernetes `Clusterrole` workloads into a specified namespace on a Kubernetes environment (endpoint) managed via Portainer.

---

## Example Usage
### Create Kubernetes Clusterrole from YAML
```hcl
resource "portainer_kubernetes_clusterrole" "example" {
  endpoint_id = 4
  manifest    = file("${path.module}/clusterrole.yaml")
}
```

## Lifecycle & Behavior
The Clusterrole is created via the Portainer Kubernetes API.

Any change results in a delete + create.

To update the Clusterrole (e.g. name, image), simply modify the manifest and re-apply:

```sh
terraform apply
```

To remove the clusterrole:
```sh
terraform destroy
```

### Arguments Reference
| Name        | Type   | Required | Description                                                  |
|-------------|--------|----------|--------------------------------------------------------------|
| endpoint_id | int    | ✅ yes   | ID of the Portainer environment (Kubernetes cluster).        |
| manifest    | string | ✅ yes   | Kubernetes Clusterrole manifest (JSON or YAML as a string).      |

---

### Attributes Reference
| Name | Description                               |
|------|-------------------------------------------|
| `id` | 	ID in the format endpoint_id:namespace:clusterrole:name    |

## Import

Kubernetes ClusterRole resources can be imported using the composite ID `endpointID:name`:

```shell
terraform import portainer_kubernetes_clusterrole.example 1:my-clusterrole
```

After import, set the `manifest` field in config to match the live object — Read only confirms the resource exists and restores identity fields, it does not reconstruct the manifest. If `manifest` is left blank after import, the next `terraform apply` will treat it as a change and may recreate the resource.
