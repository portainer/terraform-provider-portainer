variable "workflow_id" {
  description = "Identifier of a GitOps workflow on the reconcile engine"
  type        = number
  default     = 1
}

variable "source_revision" {
  description = "Source revision whose change asks for another reconcile"
  type        = string
  default     = "main"
}

resource "portainer_gitops_workflow_reconcile" "now" {
  workflow_id = var.workflow_id

  triggers = {
    revision = var.source_revision
  }
}
