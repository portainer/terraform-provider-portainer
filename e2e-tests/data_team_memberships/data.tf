# Creates a team with one member, then reads it back and checks the leader is
# identified by role.

resource "portainer_user" "member" {
  username = "e2e-team-member"
  password = var.user_password
  role     = 2
}

resource "portainer_team" "test" {
  name = "e2e-memberships-team"
}

resource "portainer_team_membership" "leader" {
  team_id = portainer_team.test.id
  user_id = portainer_user.member.id
  role    = 1 # team leader
}

data "portainer_team_memberships" "test" {
  team_id = portainer_team.test.id

  depends_on = [portainer_team_membership.leader]
}

output "leader_user_ids" {
  value = data.portainer_team_memberships.test.leader_user_ids

  precondition {
    condition     = contains(data.portainer_team_memberships.test.leader_user_ids, portainer_user.member.id)
    error_message = "the member added with role 1 must be reported as a team leader."
  }
}
