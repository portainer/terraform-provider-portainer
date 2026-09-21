# Data Source Documentation: `portainer_recommendations`

# portainer_recommendations

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Lists the recommendations Portainer has for the instance, with its own roll-up of the counts.

## Example Usage

```hcl
data "portainer_recommendations" "high" {
  severity = "high"
}

output "high_severity_recommendations" {
  value = [for r in data.portainer_recommendations.high.recommendations : r.title]
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `category` | string | ❌ no | Only list recommendations in this category. Leave unset for all of them. |
| `severity` | string | ❌ no | Only list recommendations of this severity. Leave unset for all of them. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `category_counts` | map(string) | Recommendation count per category. |
| `recommendations` | list(object) | The recommendations Portainer has for this instance. |
| `severity_counts` | map(string) | Recommendation count per severity. |
| `total` | number | Total recommendations, from Portainer's own summary rather than the filtered listing above. |
| `total_types` | number | Distinct recommendation types. |

### `recommendations`

| Name | Type | Description |
|------|------|-------------|
| `action_label` | string | Label of the action Portainer's UI offers for it. |
| `action_url` | string | Where that action leads. |
| `category` | string | Category the recommendation belongs to. |
| `description` | string | What the recommendation is about. |
| `severity` | string | How serious Portainer considers it. |
| `title` | string | Short title of the recommendation. |
| `type_id` | string | Stable identifier of the recommendation type, which is what to match on rather than the title. |

`category` and `severity` narrow the listing but not the counts: `total`, `severity_counts` and `category_counts` come from Portainer's own summary and are over every recommendation. Match on `type_id` rather than `title`, which is display text.
