# Data Source Documentation: `portainer_motd`

# portainer_motd
The `portainer_motd` data source reads Portainer's message of the day — the banner shown to users after they log in.

## Example Usage

```hcl
data "portainer_motd" "this" {}

output "portainer_announcement" {
  value = data.portainer_motd.this.message
}
```

## Arguments Reference

This data source takes no arguments.

## Attributes Reference

| Name      | Type   | Description                                                                                                     |
|-----------|--------|-----------------------------------------------------------------------------------------------------------------|
| `title`   | string | Title of the message, empty when Portainer is serving none.                                                      |
| `message` | string | Body of the message.                                                                                             |
| `style`   | string | Inline CSS Portainer's UI applies to the message.                                                                |
| `hash`    | string | Hash the UI uses to remember a user has dismissed this particular message. Changes whenever the message changes. |
