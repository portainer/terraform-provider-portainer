# Sets a tag on the environment through the bulk relations endpoint.
#
# Only tag_ids is configured on purpose: an empty list would tell Portainer to
# CLEAR that environment's edge groups, so omitting the attribute is the only
# way to leave them alone.

resource "portainer_tag" "relations_tag" {
  name = "e2e-relations-tag"
}

resource "portainer_endpoint_relations" "test" {
  relation {
    endpoint_id = var.endpoint_id
    tag_ids     = [portainer_tag.relations_tag.id]
  }
}

output "relations_applied" {
  value = portainer_endpoint_relations.test.id
}
