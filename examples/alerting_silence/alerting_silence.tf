resource "portainer_alerting_silence" "test" {
  alert_manager_url = var.alert_manager_url
  comment           = var.silence_comment
  starts_at         = var.silence_starts_at
  ends_at           = var.silence_ends_at

  matchers {
    name     = var.matcher_name
    value    = var.matcher_value
    is_regex = var.matcher_is_regex
    is_equal = var.matcher_is_equal
  }
}
