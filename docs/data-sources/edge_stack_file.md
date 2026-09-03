# Data Source Documentation: `portainer_edge_stack_file`

# portainer_edge_stack_file
Reads the stack file stored behind an edge stack, which is what Portainer distributes to the edge agents.

## Example Usage

```hcl
data "portainer_edge_stack_file" "fleet" {
  edge_stack_id = portainer_edge_stack.fleet.id
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `edge_stack_id` | number | ✅ yes | Identifier of the edge stack whose stack file is read. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `file_content` | string | Content of the stored file, exactly as Portainer holds it. |
