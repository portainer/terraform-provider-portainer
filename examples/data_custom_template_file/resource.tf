data "portainer_custom_template_file" "web" {
  template_id = 3
}

output "web_template_body" {
  value = data.portainer_custom_template_file.web.file_content
}
