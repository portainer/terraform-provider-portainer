resource "portainer_cloud_provider_provision" "do_cluster" {
  cloud_provider = var.cloud_provider

  payload = {
    credentialID      = var.do_credential_id
    name              = var.do_name
    region            = var.do_region
    nodeCount         = var.do_node_count
    nodeSize          = var.do_node_size
    networkID         = var.do_network_id
    kubernetesVersion = var.do_kubernetes_version
    meta = jsonencode({
      customTemplateID = 0
      groupId          = var.do_group_id
      stackName        = var.do_stack_name
      tagIds           = var.do_tag_ids
    })
  }
}
