resource "portainer_user_memberships_sync" "alice" {
  user_id = 1

  triggers = {
    # Change this to synchronise again.
    round = "1"
  }
}
