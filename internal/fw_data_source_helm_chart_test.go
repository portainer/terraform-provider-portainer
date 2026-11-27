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

// TestHelmChartDataSource_IndexAndChart covers both shapes: the repository
// index when no chart is named, and the per-chart command otherwise
// (defaulting to "values").
func TestHelmChartDataSource_IndexAndChart(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/templates/helm", RespondString(http.StatusOK, "text/yaml", "apiVersion: v1\nentries: {}\n"))
	mock.On("GET", "/templates/helm/values", RespondString(http.StatusOK, "text/yaml", "replicaCount: 1\n"))
	mock.On("GET", "/templates/helm/readme", RespondString(http.StatusInternalServerError, "text/plain", "boom"))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config:      mockProviderConfig(mock) + `data "portainer_helm_chart" "c" {` + "\n" + `repo = "r"` + "\n" + `chart = "nginx"` + "\n" + `command = "readme"` + "\n}",
				ExpectError: regexp.MustCompile(`status 500: boom`),
			},
			{
				Config:      mockProviderConfig(mock) + `data "portainer_helm_chart" "c" {` + "\n" + `repo = "r"` + "\n" + `command = "manifest"` + "\n}",
				ExpectError: regexp.MustCompile(`value must be one of`),
			},
			{
				Config: mockProviderConfig(mock) + `
data "portainer_helm_chart" "index" { repo = "https://charts.example.com" }
data "portainer_helm_chart" "chart" {
  repo    = "https://charts.example.com"
  chart   = "nginx"
  version = "1.2.3"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.portainer_helm_chart.index", "content", "apiVersion: v1\nentries: {}\n"),
					resource.TestCheckResourceAttr("data.portainer_helm_chart.index", "id", "helm-index-https://charts.example.com"),
					resource.TestCheckResourceAttr("data.portainer_helm_chart.chart", "content", "replicaCount: 1\n"),
					resource.TestCheckResourceAttr("data.portainer_helm_chart.chart", "command", "values"),
					resource.TestCheckResourceAttr("data.portainer_helm_chart.chart", "id", "helm-values-https://charts.example.com-nginx"),
					func(*terraform.State) error {
						get := mock.FindRequest("GET", "/templates/helm/values")
						for _, want := range []string{"chart=nginx", "version=1.2.3", "repo="} {
							if get == nil || !strings.Contains(get.Query, want) {
								return fmt.Errorf("query should contain %q, got %+v", want, get)
							}
						}
						return nil
					},
				),
			},
		},
	})
}
