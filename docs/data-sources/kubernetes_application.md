# Data Source Documentation: `portainer_kubernetes_application`

# portainer_kubernetes_application

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Inspects one application running in a Kubernetes cluster.

## Example Usage

```hcl
data "portainer_kubernetes_application" "web" {
  endpoint_id = portainer_environment.prod.id
  namespace   = "apps"
  name        = "web"
}

output "web_pods_missing" {
  value = data.portainer_kubernetes_application.web.total_pods - data.portainer_kubernetes_application.web.running_pods
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `endpoint_id` | number | ✅ yes | Identifier of the Kubernetes environment to query. |
| `name` | string | ✅ yes | Name of the application to inspect. |
| `namespace` | string | ✅ yes | Namespace of the application. |
| `resource_type` | string | ❌ no | Kind to look the application up as, for example `Deployment`. Leave unset to let Portainer work it out. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `application_owner` | string | Who Portainer records as the application's owner. |
| `application_type` | string | How Portainer classifies the application. |
| `creation_date` | string | When the application was created. |
| `deployment_type` | string | How the application was deployed. |
| `details` | string | The whole response as Portainer returns it, encoded as JSON. Decode it with `jsondecode()` for the pod, container and resource detail the attributes above do not cover. |
| `image` | string | Container image the application runs. |
| `kind` | string | Kind of the workload, for example `Deployment` or `StatefulSet`. |
| `labels` | map(string) | Labels on the application. |
| `load_balancer_ip` | string | External address of the load balancer, empty until one is assigned. |
| `running_pods` | number | Pods currently running. |
| `service_name` | string | Service fronting the application, empty when it has none. |
| `service_type` | string | Type of that service, for example `ClusterIP` or `LoadBalancer`. |
| `stack_id` | string | Stack the application belongs to, empty when it is not part of one. |
| `stack_name` | string | Name of that stack. |
| `status` | string | Status Portainer reports for the application. |
| `total_pods` | number | Pods the application asks for. A gap between this and `running_pods` is what to watch. |
| `uid` | string | Kubernetes UID of the application. |

A gap between `total_pods` and `running_pods` is what to watch. Pods, containers, published ports and resource figures nest several levels deep and differ by workload kind, so `details` carries the whole response as JSON alongside the fields above.
