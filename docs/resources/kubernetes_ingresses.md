# ⚙️ **Resource Documentation: `portainer_kubernetes_ingress`**

# portainer_kubernetes_ingress

The `portainer_kubernetes_ingress` resource allows you to deploy one-off Kubernetes `Ingress` workloads into a specified namespace on a Kubernetes environment (endpoint) managed via Portainer.

---

## 🧩 Example Usage

```hcl
resource "portainer_kubernetes_ingress" "example" {
  environment_id = 4
  namespace      = "default"
  name           = "example-ingress"
  class_name     = "nginx"

  annotations = {
    "nginx.ingress.kubernetes.io/rewrite-target" = "/"
  }

  labels = {
    "app" = "frontend"
  }

  hosts = ["example.com"]

  tls {
    hosts       = ["example.com"]
    secret_name = "example-tls"
  }

  paths {
    host         = "example.com"
    path         = "/"
    path_type    = "Prefix"
    port         = 80
    service_name = "frontend-service"
  }
}
```

---

## ⚙️ Lifecycle & Behavior

* The ingress is managed through the **Portainer Kubernetes API**.
* Any change to configuration (e.g., annotations, hosts, paths) **forces a delete + recreate**.
* To apply changes:

  ```sh
  terraform apply
  ```
* To remove the ingress:

  ```sh
  terraform destroy
  ```
* Currently, deletion via Portainer API is **not yet supported**, so Terraform may only update and recreate.

---

## 🧾 Arguments Reference

| Name             | Type         | Required    | Description                                                                    |
| ---------------- | ------------ | ----------- | ------------------------------------------------------------------------------ |
| `environment_id` | int          | ✅ yes       | ID of the Portainer environment (Kubernetes cluster).                         |
| `namespace`      | string       | ✅ yes       | Kubernetes namespace where the Ingress should be created.                     |
| `name`           | string       | ✅ yes       | Name of the Kubernetes Ingress.                                               |
| `class_name`     | string       | 🚫 optional | The ingress class name (e.g. `nginx`, `traefik`).                              |
| `hosts`          | list(string) | 🚫 optional | List of hostnames associated with the ingress.                                 |
| `annotations`    | map(string)  | 🚫 optional | Key/value pairs to annotate the Ingress with.                                  |
| `labels`         | map(string)  | 🚫 optional | Key/value pairs of Kubernetes labels to apply to the Ingress.                  |
| `tls`            | block(list)  | 🚫 optional | Defines TLS configuration for the ingress.                                     |
| `paths`          | block(list)  | 🚫 optional | Defines routing paths and backend services.                                    |

### `tls` block
| Attribute     | Type         | Description                                                                     |
| ------------- | ------------ | ------------------------------------------------------------------------------- |
| `hosts`       | list(string) | List of hostnames included in this TLS configuration (e.g., `["example.com"]`). |
| `secret_name` | string       | Name of the Kubernetes Secret containing the TLS certificate and key.           |

### `paths` block
| Attribute      | Type   | Description                                                |
| -------------- | ------ | ---------------------------------------------------------- |
| `host`         | string | The hostname to match for this path (e.g., `example.com`). |
| `path`         | string | The URL path to route (e.g., `/`, `/api`).                 |
| `path_type`    | string | Type of path matching (`Prefix`, `Exact`, etc.).           |
| `port`         | int    | Target service port number.                                |
| `service_name` | string | Name of the Kubernetes Service that receives the traffic.  |

---

## 📦 Attributes Reference

| Name | Description                                                      |
| ---- | ---------------------------------------------------------------- |
| `id` | ID of the ingress in the format `environment_id:namespace:name`. |

## Import

Kubernetes Ingress resources can be imported using the composite ID `environmentID:namespace:name`:

```shell
terraform import portainer_kubernetes_ingresses.example 1:default:my-ingress
```

After import, set `hosts`, `paths`, `tls`, `annotations`, `labels`, and `class_name` in config to match the live Ingress — Read only confirms it exists and restores identity fields, it does not reconstruct the routing spec.
