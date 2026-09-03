data "portainer_edge_job_file" "nightly" {
  edge_job_id = 1
}

output "nightly_script" {
  value = data.portainer_edge_job_file.nightly.file_content
}
