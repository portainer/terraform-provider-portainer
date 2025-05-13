resource "portainer_custom_template" "example_string" {
  title             = var.custom_template_title
  description       = var.custom_template_description
  note              = var.custom_template_note
  platform          = var.custom_template_platform
  type              = var.custom_template_type
  edge_template     = var.custom_template_edge
  is_compose_format = var.custom_template_is_compose
  file_content      = var.custom_template_file_content
}
