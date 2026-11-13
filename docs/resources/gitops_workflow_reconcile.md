# Resource Documentation: `portainer_gitops_workflow_reconcile`

# portainer_gitops_workflow_reconcile

> **Requires Portainer Business Edition 3.0 or later.** On Community Edition or an older server the reconcile fails with an error that says so. It only applies to a workflow on the `reconcile` engine.


Asks a GitOps workflow on the reconcile engine to read its sources again and every environment running it to reconcile now, instead of waiting for its `reconcile_interval`. That applies whatever the sources now describe and puts back anything that has drifted - also when `drift_mode` only reports drift.

## Example Usage

```hcl
resource "portainer_gitops_workflow_reconcile" "now" {
  workflow_id = portainer_gitops_workflow.apps.workflow_id

  triggers = {
    revision = var.source_revision
  }
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `workflow_id` | number | ✅ yes | Identifier of the GitOps workflow to reconcile. Changing it forces a new resource. |
| `triggers` | map | ❌ no | Arbitrary values that force another reconcile when they change. |

## Attributes Reference

This resource exports no attributes beyond its `id`.

## Lifecycle & Behavior

A reconcile is a one-shot action, so without a `triggers` value the resource runs once and then stays put. Destroying it only removes it from state.

Portainer does not wait for the environments: they re-apply at their next check-in. A successful apply means the request was accepted, not that every environment has applied it - read the workflow's status for that.

On a server older than Portainer Business Edition 3.0 the apply fails with an error naming the requirement.
