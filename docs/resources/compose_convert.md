# 🔁 **Resource Documentation: portainer_compose_convert**

# portainer_compose_convert
The `portainer_compose_convert` resource allows you to convert a Docker Compose configuration into Kubernetes YAML manifests using [Kompose](https://github.com/kubernetes/kompose).

> ⚠️ Note: This resource **only performs conversion**. It does not apply or deploy anything to Kubernetes or Portainer.

> 💡 Kompose must be available in your environment. You can install it by following the [Kompose installation guide](https://github.com/kubernetes/kompose/blob/main/docs/installation.md).

---

## 📌 Example Usage

```hcl
resource "portainer_compose_convert" "example" {
  compose_content = file("${path.module}/docker-compose.yml")
}

resource "local_file" "k8s_manifests" {
  for_each = portainer_compose_convert.example.manifests

  filename = "${path.module}/output/${each.key}"
  content  = each.value
}
```

---

## ⚙️ Lifecycle & Behavior

- This resource runs **Kompose conversion** when applied.
- It creates a temporary directory, writes the Compose content into it, and invokes `kompose convert`.
- The generated Kubernetes YAML manifests are returned as a map (`filename → content`) via the `manifests` output.
- The resource is **always re-evaluated on content change**, but otherwise does not manage external state.

---

## 🧾 Arguments Reference

| Name              | Type   | Required | Description                                                                 |
|-------------------|--------|----------|-----------------------------------------------------------------------------|
| `compose_content` | string | ✅ yes   | The content of your `docker-compose.yml` file as a string. You can inline or use `file(...)`. |

---

## 📄 Attributes Reference

| Name        | Type               | Description                                                      |
|-------------|--------------------|------------------------------------------------------------------|
| `id`        | string             | Internal identifier for the conversion run (auto-generated).     |
| `manifests` | map(string)        | Map of generated Kubernetes YAML manifest filenames to content.  |

---

## 📌 Kompose Requirement

Kompose must be installed or available via Docker container. See:
👉 [https://github.com/kubernetes/kompose/blob/main/docs/installation.md](https://github.com/kubernetes/kompose/blob/main/docs/installation.md)