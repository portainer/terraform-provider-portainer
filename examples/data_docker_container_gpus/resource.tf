data "portainer_docker_container_gpus" "trainer" {
  environment_id = var.environment_id
  container_id   = "b3f1a2c4d5e6"
}

output "trainer_gpus" {
  value = data.portainer_docker_container_gpus.trainer.gpus
}
