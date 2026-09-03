resource "portainer_endpoint_relations" "fleet" {
  relation {
    endpoint_id    = 9
    edge_group_ids = [1, 2]
    tag_ids        = [3]
    group_id       = 5
  }

  # Only the edge groups are managed here. tag_ids and group_id are omitted on
  # purpose: an empty list would tell Portainer to CLEAR that environment's
  # tags, and omitting the attribute is the only way to leave them alone.
  relation {
    endpoint_id    = 10
    edge_group_ids = [1]
  }
}
