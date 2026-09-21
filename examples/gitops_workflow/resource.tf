resource "portainer_gitops_workflow" "platform" {
  name       = "platform"
  on_destroy = "destroy"

  artifact {
    name            = "monitoring"
    type            = "edgeStack"
    deployment_type = "compose"
    edge_group_ids  = [1]

    file {
      source_id = 1
      path      = "monitoring/portainer.yaml"
      ref       = "refs/heads/main"
    }

    config {
      pre_pull_image = true

      environment = {
        LOG_LEVEL = "info"
      }

      parallel {
        batch_count    = 5
        delay          = "30"
        failure_action = "continue"
      }
    }
  }
}

output "gitops_workflow_id" {
  value = portainer_gitops_workflow.platform.workflow_id
}
