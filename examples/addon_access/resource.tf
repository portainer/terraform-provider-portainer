resource "portainer_addon_access" "portal" {
  addon_id = "portal-template"
  enabled  = true

  user_access {
    user_id = 3
    role_id = 2
  }

  team_access {
    team_id = 5
    role_id = 1
  }
}
