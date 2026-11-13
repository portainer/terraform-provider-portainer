# A workflow on the 3.0 reconcile engine, deploying a public manifest to k3d.
resource "portainer_gitops_source" "e2e" {
  url      = "https://github.com/stefanprodan/podinfo"
  name     = "e2e-reconcile"
  interval = "1h"
  public   = true
}

resource "portainer_gitops_workflow" "e2e" {
  name               = "e2e-reconcile"
  engine             = "reconcile"
  reconcile_interval = "10m"
  on_destroy         = "destroy"

  artifact {
    name            = "podinfo"
    deployment_type = "kubernetes"
    environment_ids = [var.k8s_environment_id]

    file {
      source_id = tonumber(portainer_gitops_source.e2e.id)
      path      = "kustomize/deployment.yaml"
      ref       = "refs/heads/master"
    }

    config {
      drift_mode       = "warn"
      kubernetes_prune = true
    }
  }
}

resource "portainer_gitops_workflow_reconcile" "e2e" {
  workflow_id = portainer_gitops_workflow.e2e.workflow_id
  triggers    = { run = "1" }
}
