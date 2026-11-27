package internal

import (
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestHelmReleaseHistoryDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/4/kubernetes/helm/myapp/history", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"name": "myapp", "namespace": "default", "version": 1, "appVersion": "1.0.0",
			"info": map[string]interface{}{
				"status": "superseded", "description": "Install complete", "last_deployed": "2024-01-01T00:00:00Z",
			},
			"chart": map[string]interface{}{"metadata": map[string]interface{}{"name": "myapp", "version": "0.1.0"}},
		},
		{
			"version": 2,
			"info":    map[string]interface{}{"status": "deployed"},
			"chart":   map[string]interface{}{"metadata": map[string]interface{}{"name": "myapp"}},
		},
	}))
	const ds = "data.portainer_helm_release_history.h"
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
data "portainer_helm_release_history" "h" {
  endpoint_id  = 4
  release_name = "myapp"
}
`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(ds, "id", "helm-history-4-myapp"),
				resource.TestCheckResourceAttr(ds, "revisions.#", "2"),
				resource.TestCheckResourceAttr(ds, "revisions.0.revision", "1"),
				resource.TestCheckResourceAttr(ds, "revisions.0.status", "superseded"),
				resource.TestCheckResourceAttr(ds, "revisions.0.chart", "myapp-0.1.0"),
				resource.TestCheckResourceAttr(ds, "revisions.0.app_version", "1.0.0"),
				resource.TestCheckResourceAttr(ds, "revisions.0.updated", "2024-01-01T00:00:00Z"),
				resource.TestCheckResourceAttr(ds, "revisions.1.revision", "2"),
				resource.TestCheckResourceAttr(ds, "revisions.1.chart", "myapp"),
				resource.TestCheckResourceAttr(ds, "revisions.1.name", ""),
				resource.TestCheckResourceAttr(ds, "revisions.1.app_version", ""),
			),
		}},
	})
}

func TestHelmReleaseHistoryDataSource_NamespaceAndError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/2/kubernetes/helm/app/history", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("GET", "/endpoints/1/kubernetes/helm/missing/history", RespondString(http.StatusNotFound, "application/json", `{"message":"release not found"}`))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + `data "portainer_helm_release_history" "h" {` + "\n" + `endpoint_id = 2` + "\n" + `release_name = "app"` + "\n" + `namespace = "prod"` + "\n}",
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.portainer_helm_release_history.h", "revisions.#", "0"),
					func(*terraform.State) error {
						req := mock.FindRequest("GET", "/endpoints/2/kubernetes/helm/app/history")
						if req == nil || req.Query != "namespace=prod" {
							return fmt.Errorf("expected query namespace=prod, got %+v", req)
						}
						return nil
					},
				),
			},
			{
				Config:      mockProviderConfig(mock) + `data "portainer_helm_release_history" "h" {` + "\n" + `endpoint_id = 1` + "\n" + `release_name = "missing"` + "\n}",
				ExpectError: regexp.MustCompile(`failed to get Helm release history`),
			},
		},
	})
}
