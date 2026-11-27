package internal

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestEnvironmentLogsDataSource_BuildsQuery pins the window and the filters,
// which are the whole interface of this data source.
func TestEnvironmentLogsDataSource_BuildsQuery(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/observability/environments/5/logs", RespondJSON(http.StatusOK, map[string]interface{}{
		"logs": []map[string]interface{}{{
			"time": "2026-01-01T00:00:00Z", "severity": "error", "source": "web-0",
			"message": "connection refused", "labels": map[string]string{"app": "web"},
		}},
	}))

	name := "data.portainer_environment_logs.l"
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `data "portainer_environment_logs" "l" {
  endpoint_id = 5
  from        = "2026-01-01T00:00:00Z"
  to          = "2026-01-02T00:00:00Z"
  namespace   = "apps"
  severity    = "error"
  limit       = 100
}`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(name, "id", "portainer-environment-logs-5"),
				resource.TestCheckResourceAttr(name, "logs.#", "1"),
				resource.TestCheckResourceAttr(name, "logs.0.message", "connection refused"),
				resource.TestCheckResourceAttr(name, "logs.0.severity", "error"),
				resource.TestCheckResourceAttr(name, "logs.0.labels.app", "web"),
				func(_ *terraform.State) error {
					query := mock.FindRequest("GET", "/observability/environments/5/logs").Query
					for _, want := range []string{"from=2026", "to=2026", "namespace=apps", "severity=error", "limit=100"} {
						if !strings.Contains(query, want) {
							return fmt.Errorf("expected %q in the query, got %q", want, query)
						}
					}
					// skip was not set, so it must not be sent.
					if strings.Contains(query, "skip=") || strings.Contains(query, "kind=") {
						return fmt.Errorf("unset filters must not be sent, got %q", query)
					}
					return nil
				},
			),
		}},
	})
}

// TestEnvironmentMetricsDataSource_CarriesSeriesAsJSON pins the decision to
// pass the series through: their shape depends on the metric and grouping.
// A window with no data reads as an empty JSON array.
func TestEnvironmentMetricsDataSource_CarriesSeriesAsJSON(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/observability/environments/5/metrics", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("metric") == "memory" {
			RespondJSON(http.StatusOK, map[string]interface{}{})(w, r)
			return
		}
		RespondJSON(http.StatusOK, map[string]interface{}{
			"data": []map[string]interface{}{{"namespace": "apps", "value": 0.42}},
		})(w, r)
	})

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
data "portainer_environment_metrics" "cpu" {
  endpoint_id = 5
  metric      = "cpu"
  aggregation = "avg"
  from        = "2026-01-01T00:00:00Z"
  to          = "2026-01-02T00:00:00Z"
  group_by    = "namespace"
}
data "portainer_environment_metrics" "memory" {
  endpoint_id = 5
  metric      = "memory"
  aggregation = "avg"
  from        = "a"
  to          = "b"
}
`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("data.portainer_environment_metrics.cpu", "id", "portainer-environment-metrics-5-cpu"),
				resource.TestMatchResourceAttr("data.portainer_environment_metrics.cpu", "data", regexp.MustCompile(`0\.42`)),
				resource.TestCheckResourceAttr("data.portainer_environment_metrics.memory", "data", "[]"),
				func(_ *terraform.State) error {
					for _, r := range mock.Requests() {
						if !strings.Contains(r.Query, "metric=cpu") {
							continue
						}
						for _, want := range []string{"aggregation=avg", "groupBy=namespace"} {
							if !strings.Contains(r.Query, want) {
								return fmt.Errorf("expected %q in the query, got %q", want, r.Query)
							}
						}
						return nil
					}
					return fmt.Errorf("the cpu query was not sent")
				},
			),
		}},
	})
}
