package internal

import (
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestImageStatusDataSource_KindSelectsEndpoint pins the three endpoints this
// data source folds, including the stack one that has no environment in its
// path.
func TestImageStatusDataSource_KindSelectsEndpoint(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		kind, resourceID, path, refresh, query string
	}{
		{"container", "abc123", "/docker/5/containers/abc123/image_status", "true", "refresh=true"},
		{"service", "svc-1", "/docker/5/services/svc-1/image_status", "false", ""},
		{"stack", "9", "/stacks/9/images_status", "true", "refresh=true"},
	} {
		mock := NewMockServer(t)
		mock.On("GET", tc.path, RespondJSON(http.StatusOK, map[string]interface{}{
			"Status": "outdated", "Message": "a newer image is published",
		}))
		const ds = "data.portainer_image_status.s"
		resource.UnitTest(t, resource.TestCase{
			ProtoV5ProviderFactories: testProtoV5Factories,
			Steps: []resource.TestStep{{
				Config: mockProviderConfig(mock) + fmt.Sprintf(`
data "portainer_image_status" "s" {
  kind        = %q
  endpoint_id = 5
  resource_id = %q
  refresh     = %s
}
`, tc.kind, tc.resourceID, tc.refresh),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(ds, "status", "outdated"),
					resource.TestCheckResourceAttr(ds, "message", "a newer image is published"),
					resource.TestCheckResourceAttr(ds, "id", fmt.Sprintf("portainer-image-status-%s-5-%s", tc.kind, tc.resourceID)),
					func(*terraform.State) error {
						req := mock.FindRequest("GET", tc.path)
						if req == nil || req.Query != tc.query {
							return fmt.Errorf("%s: expected GET %s?%s, got %+v", tc.kind, tc.path, tc.query, req)
						}
						return nil
					},
				),
			}},
		})
	}
}

// TestImageStatusDataSource_RequiresEndpointForContainer catches the
// configuration mistake the folded data source makes possible; refresh
// defaults to false.
func TestImageStatusDataSource_RequiresEndpointForContainer(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/stacks/9/images_status", RespondJSON(http.StatusOK, map[string]interface{}{"Status": "updated"}))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config:      mockProviderConfig(mock) + `data "portainer_image_status" "s" {` + "\n" + `kind = "container"` + "\n" + `resource_id = "abc123"` + "\n}",
				ExpectError: regexp.MustCompile(`endpoint_id is required when kind is "container"`),
			},
			{
				Config:      mockProviderConfig(mock) + `data "portainer_image_status" "s" {` + "\n" + `kind = "volume"` + "\n" + `resource_id = "abc123"` + "\n}",
				ExpectError: regexp.MustCompile(`value must be one of`),
			},
			{
				Config: mockProviderConfig(mock) + `data "portainer_image_status" "s" {` + "\n" + `kind = "stack"` + "\n" + `resource_id = "9"` + "\n}",
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.portainer_image_status.s", "refresh", "false"),
					resource.TestCheckResourceAttr("data.portainer_image_status.s", "id", "portainer-image-status-stack-0-9"),
				),
			},
		},
	})
}
