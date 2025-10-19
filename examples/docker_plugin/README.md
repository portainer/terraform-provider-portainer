<!-- BEGIN_TF_DOCS -->


## Providers

| Name | Version |
|------|---------|
| <a name="provider_portainer"></a> [portainer](#provider\_portainer) | 1.13.2 |

## Resources

| Name | Type |
|------|------|
| [portainer_docker_plugin.rclone](https://registry.terraform.io/providers/portainer/portainer/latest/docs/resources/docker_plugin) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_endpoint_id"></a> [endpoint\_id](#input\_endpoint\_id) | ID of the environment where the network will be created | `number` | n/a | yes |
| <a name="input_name"></a> [name](#input\_name) | Local alias name under which the plugin will be registered (e.g. rclone) | `string` | `"rclone"` | no |
| <a name="input_portainer_api_key"></a> [portainer\_api\_key](#input\_portainer\_api\_key) | Default Portainer Admin API Key | `string` | n/a | yes |
| <a name="input_portainer_skip_ssl_verify"></a> [portainer\_skip\_ssl\_verify](#input\_portainer\_skip\_ssl\_verify) | Set to true to skip TLS certificate verification (useful for self-signed certs) | `bool` | `true` | no |
| <a name="input_portainer_url"></a> [portainer\_url](#input\_portainer\_url) | Default Portainer URL | `string` | n/a | yes |
| <a name="input_remote"></a> [remote](#input\_remote) | Remote reference of the plugin to install (e.g. rclone/docker-volume-rclone) | `string` | `"rclone/docker-volume-rclone"` | no |
| <a name="input_settings"></a> [settings](#input\_settings) | List of plugin permission settings required for rclone/docker-volume-rclone plugin.<br/>Each object must define:<br/>  - name: setting type (e.g. "network", "mount", "device", "capabilities")<br/>  - value: list of string values for the setting<br/><br/>Defaults correspond to:<br/>  - network: ["host"]<br/>  - mount: [/var/lib/docker-plugins/rclone/config, /var/lib/docker-plugins/rclone/cache]<br/>  - device: [/dev/fuse]<br/>  - capabilities: [CAP\_SYS\_ADMIN] | <pre>list(object({<br/>    name  = string<br/>    value = list(string)<br/>  }))</pre> | <pre>[<br/>  {<br/>    "name": "network",<br/>    "value": [<br/>      "host"<br/>    ]<br/>  },<br/>  {<br/>    "name": "mount",<br/>    "value": [<br/>      "/var/lib/docker-plugins/rclone/config"<br/>    ]<br/>  },<br/>  {<br/>    "name": "mount",<br/>    "value": [<br/>      "/var/lib/docker-plugins/rclone/cache"<br/>    ]<br/>  },<br/>  {<br/>    "name": "device",<br/>    "value": [<br/>      "/dev/fuse"<br/>    ]<br/>  },<br/>  {<br/>    "name": "capabilities",<br/>    "value": [<br/>      "CAP_SYS_ADMIN"<br/>    ]<br/>  }<br/>]</pre> | no |
<!-- END_TF_DOCS -->