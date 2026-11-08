resource "portainer_kubernetes_configmaps" "drift" {
  endpoint_id = var.endpoint_id
  namespace   = "default"
  manifest    = <<-YAML
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: e2e-drift-config
    data:
      log_level: info
    %{if var.with_feature}
      feature: "on"
    %{endif}
  YAML
}

# The limits use the forms the API server rewrites (0.5 -> 500m), which must
# not show up as drift.
resource "portainer_kubernetes_application" "drift" {
  endpoint_id = var.endpoint_id
  namespace   = "default"
  manifest    = <<-YAML
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: e2e-drift-app
      labels:
        app: e2e-drift-app
    spec:
      replicas: 1
      selector:
        matchLabels:
          app: e2e-drift-app
      template:
        metadata:
          labels:
            app: e2e-drift-app
        spec:
          containers:
            - name: web
              image: ${var.image}
              resources:
                limits:
                  cpu: 0.5
                  memory: 64Mi
  YAML
}
