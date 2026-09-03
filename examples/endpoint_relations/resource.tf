resource "portainer_endpoint_relations" "fleet" {
  relation {
    endpoint_id    = 9
    edge_group_ids = [1, 2]
    tag_ids        = [3]
    group_id       = 5
  }

  # group_id left at 0 keeps the environment in whatever group it is already in.
  relation {
    endpoint_id    = 10
    edge_group_ids = [1]
  }
}
