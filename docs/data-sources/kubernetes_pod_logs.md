# Data Source Documentation: `portainer_kubernetes_pod_logs`

# portainer_kubernetes_pod_logs
The `portainer_kubernetes_pod_logs` data source reads the log output of a Kubernetes pod through Portainer, the equivalent of `kubectl logs`. It is most useful for surfacing why a workload failed as part of a plan or an output, rather than having to open the UI.

The API's `follow` parameter is deliberately **not** exposed: it keeps the response open until the client disconnects, which would hang a plan or an apply.

Requires Portainer **2.45.0** or newer.

## Example Usage

```hcl
data "portainer_kubernetes_pod_logs" "web" {
  environment_id = 1
  namespace      = "default"
  pod_name       = "web-6b8f7d9c4d-abcde"
  container      = "web"
  tail_lines     = 200
  timestamps     = true
}

output "web_log_tail" {
  value = data.portainer_kubernetes_pod_logs.web.logs
}
```

### Debug a crash-looping container

```hcl
data "portainer_kubernetes_pod_logs" "crashed" {
  environment_id = 1
  namespace      = "default"
  pod_name       = "web-6b8f7d9c4d-abcde"
  container      = "web"
  previous       = true
}
```

## Arguments Reference

| Name             | Type   | Required | Default | Description                                                                                                          |
|------------------|--------|----------|---------|----------------------------------------------------------------------------------------------------------------------|
| `environment_id` | number | Yes      | –       | Environment (endpoint) identifier of the Kubernetes environment the pod runs in.                                      |
| `namespace`      | string | Yes      | –       | Namespace the pod lives in.                                                                                           |
| `pod_name`       | string | Yes      | –       | Name of the pod to read logs from.                                                                                    |
| `container`      | string | No       | –       | Container to read logs from. Required when the pod runs more than one container.                                       |
| `tail_lines`     | number | No       | –       | Number of lines to return from the end of the log. Leave unset to return the whole available log, which can be large. |
| `since_seconds`  | number | No       | –       | Only return log lines newer than this many seconds.                                                                   |
| `timestamps`     | bool   | No       | `false` | Prefix every line with an RFC3339 timestamp.                                                                          |
| `previous`       | bool   | No       | `false` | Read the log of the previous terminated instance of the container.                                                    |

## Attributes Reference

| Name         | Type   | Description                                         |
|--------------|--------|-----------------------------------------------------|
| `logs`       | string | Log output returned by the pod, as plain text.      |
| `line_count` | number | Number of non-empty lines in `logs`.                |

Logs are stored in Terraform state in plain text. Set `tail_lines` to keep the state small, and treat the state as sensitive if the workload logs secrets.
