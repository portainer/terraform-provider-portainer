resource "portainer_omni_cluster" "edge_fleet" {
  credential_id      = 1
  name               = "edge-fleet"
  talos_version      = "v1.8.0"
  kubernetes_version = "v1.31.0"

  labels = {
    environment = "production"
  }

  control_plane {
    machine {
      name             = "cp-1"
      hostname         = "cp-1"
      install_disk     = "/dev/sda"
      nameservers      = ["1.1.1.1"]
      system_disk_size = 50

      interface {
        interface = "eth0"
        addresses = ["10.0.0.11/24"]

        route {
          network = "0.0.0.0/0"
          gateway = "10.0.0.1"
        }
      }
    }
  }

  worker {
    name = "pool-a"

    machine {
      name         = "worker-1"
      install_disk = "/dev/sda"

      interface {
        interface = "eth0"
        dhcp      = true
      }

      user_disk {
        volume_name = "data"
        size        = 0
      }
    }
  }
}

output "omni_cluster_environment_id" {
  value = portainer_omni_cluster.edge_fleet.endpoint_id
}
