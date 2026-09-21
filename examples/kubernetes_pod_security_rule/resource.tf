resource "portainer_kubernetes_pod_security_rule" "baseline" {
  endpoint_id = 1
  enabled     = true

  privileged_containers      = true
  allow_privilege_escalation = true
  host_namespaces            = true
  read_only_root_filesystem  = true

  capabilities {
    allowed       = ["NET_BIND_SERVICE"]
    required_drop = ["ALL"]
  }

  host_ports {
    host_network = false
    min          = 30000
    max          = 32767
  }

  host_filesystem {
    allowed_path {
      path_prefix = "/var/log"
      readonly    = true
    }
  }

  users {
    run_as_user {
      type = "MustRunAs"

      id_range {
        min = 1000
        max = 2000
      }
    }
  }
}
