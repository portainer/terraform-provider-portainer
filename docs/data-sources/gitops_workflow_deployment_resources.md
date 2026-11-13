# Data Source Documentation: `portainer_gitops_workflow_deployment_resources`

# portainer_gitops_workflow_deployment_resources
The `portainer_gitops_workflow_deployment_resources` data source lists the Kubernetes resources a reconciled deployment of a GitOps workflow declares. They are read from the manifests Portainer materialised for the deployment - one copy of what every targeted environment applies, at the revision the deployment last assembled, not at the source's head.

> **Requires Portainer Business Edition 3.0 or later**, for workflows on the `reconcile` engine. A Helm artifact lists nothing: what a release contains is only known once Helm has rendered it on the cluster.

## Example Usage

```hcl
data "portainer_gitops_workflow_deployment_resources" "web" {
  workflow_id   = portainer_gitops_workflow.apps.workflow_id
  deployment_id = portainer_gitops_workflow.apps.artifact[0].artifact_id
}

output "web_deployments" {
  value = [
    for r in data.portainer_gitops_workflow_deployment_resources.web.resources :
    "${r.namespace}/${r.name}" if r.kind == "Deployment"
  ]
}
```

## Arguments Reference

| Name            | Type   | Required | Description                                                                                          |
|-----------------|--------|----------|------------------------------------------------------------------------------------------------------|
| `workflow_id`   | number | ✅ yes   | Identifier of the GitOps workflow.                                                                    |
| `deployment_id` | number | ✅ yes   | Identifier of the reconciled deployment: the `artifact_id` of an artifact on the `reconcile` engine. |

## Attributes Reference

| Name        | Type | Description                                                                                                         |
|-------------|------|---------------------------------------------------------------------------------------------------------------------|
| `resources` | list | Resources the deployment declares. Each entry has `api_version`, `kind`, `name` and `namespace` (empty when cluster-scoped). |

On a server older than Portainer Business Edition 3.0 the read fails with an error naming the requirement.
