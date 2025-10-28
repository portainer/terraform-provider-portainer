# 💬 **Resource Documentation: `portainer_chat`**

## Overview
The `portainer_chat` resource allows you to send a query to Portainer’s integrated OpenAI assistant (if enabled). It supports providing context and environment-specific prompts, and retrieves an AI-generated YAML or message.

> Currently working only for Portainer BE edition

---

## 📘 Example Usage

```hcl
resource "portainer_chat" "test" {
  context        = "environment_aware"
  environment_id = "Your environment id"
  message        = "Your text/message"
  model          = "gpt-3.5-turbo"
}

output "chat_message" {
  value = portainer_chat.test.response_message
}

output "chat_yaml" {
  value = portainer_chat.test.response_yaml
}
```

---

## ⚙️ Lifecycle & Behavior
This resource performs a one-time request via:

```
POST /chat
```

It does not update unless `message`, `context`, `environment_id` or `model` changes. Portainer returns both a message and YAML which are made available as Terraform outputs.

Note: The resource ID is synthetic (`chat-{environment_id}`) and not persisted remotely.

---

## 🧾 Arguments Reference

| Name              | Type   | Required | Description                                                            |
|-------------------|--------|----------|------------------------------------------------------------------------|
| `context`         | string | ✅ yes   | Context of the query (e.g. `environment_aware`)                        |
| `environment_id`  | number | ✅ yes   | ID of the Portainer environment                                        |
| `message`         | string | ✅ yes   | The natural language prompt to send to OpenAI                          |
| `model`           | string | 🚫 optional | The OpenAI model to use (e.g. `gpt-3.5-turbo`, `gpt-4`)                |

---

## 📤 Attributes Reference

| Name               | Description                                       |
|--------------------|---------------------------------------------------|
| `id`               | Set to `chat-{environment_id}`                   |
| `response_message` | The AI-generated natural language response       |
| `response_yaml`    | The AI-generated deployment YAML (if available)  |
