resource "portainer_custom_template" "example_string" {
  title       = var.custom_template_title
  description = var.custom_template_description
  note        = var.custom_template_note
  platform    = var.custom_template_platform
  type        = var.custom_template_type
  file_path   = "${path.module}/${var.custom_template_file_path}"
}
