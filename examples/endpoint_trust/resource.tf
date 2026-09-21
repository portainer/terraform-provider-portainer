resource "portainer_endpoint_trust" "shop_floor_3" {
  endpoint_id    = 12
  group_id       = 1
  edge_group_ids = [1]
  tag_ids        = [1]
}

output "endpoint_trust_edge_id" {
  value = portainer_endpoint_trust.shop_floor_3.edge_id
}
