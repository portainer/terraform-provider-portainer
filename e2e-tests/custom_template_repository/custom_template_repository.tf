resource "portainer_custom_template" "example_custom_template_repository" {
  title                = var.custom_template_title
  description          = var.custom_template_description
  note                 = var.custom_template_note
  platform             = var.custom_template_platform
  type                 = var.custom_template_type
  repository_url       = var.custom_template_repository_url
  repository_reference = var.custom_template_repository_reference
  compose_file_path    = var.custom_template_compose_file_path
}
