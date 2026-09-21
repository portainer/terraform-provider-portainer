# Full cycle: the environment joins the group on apply and returns to Unassigned
# on destroy.

resource "portainer_endpoint_group" "test" {
  name = "e2e-membership-group"
}

resource "portainer_endpoint_group_membership" "test" {
  endpoint_group_id = portainer_endpoint_group.test.id
  endpoint_id       = var.endpoint_id
}

output "membership_id" {
  value = portainer_endpoint_group_membership.test.id
}
