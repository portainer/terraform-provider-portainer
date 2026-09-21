# Creates a custom template, then reads its stored stack file back.

resource "portainer_custom_template" "test" {
  title        = "e2e-template-file"
  description  = "e2e"
  note         = "created by the e2e suite"
  platform     = 1
  type         = 2
  file_content = "version: '3'\nservices:\n  web:\n    image: nginx:alpine\n"
}

data "portainer_custom_template_file" "test" {
  template_id = portainer_custom_template.test.id
}

output "template_file_roundtrip" {
  value = data.portainer_custom_template_file.test.file_content

  precondition {
    condition     = length(data.portainer_custom_template_file.test.file_content) > 0
    error_message = "the stored template file must be readable back."
  }
}
