# change_window and deployment_options are Business Edition settings.
resource "portainer_endpoint_settings" "be" {
  endpoint_id = var.endpoint_id

  change_window {
    enabled    = true
    start_time = "22:00"
    end_time   = "02:00"
  }

  deployment_options {
    override_global_options = true
    hide_add_with_form      = true
    hide_file_upload        = false
    hide_web_editor         = false
  }
}
