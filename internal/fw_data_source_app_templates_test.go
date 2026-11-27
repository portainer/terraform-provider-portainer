package internal

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func appTemplatesMock(t *testing.T) *MockServer {
	mock := NewMockServer(t)
	mock.On("GET", "/templates", RespondJSON(http.StatusOK, map[string]interface{}{
		"version": "3",
		"templates": []map[string]interface{}{
			{
				"Id": 12, "title": "WordPress", "description": "Blog", "type": 3,
				"platform": "linux", "categories": []string{"cms"},
				"repository": map[string]interface{}{"url": "https://github.com/acme/templates"},
			},
			{"Id": 13, "title": "Nginx", "type": 1, "image": "nginx:latest"},
		},
	}))
	mock.On("POST", "/templates/12/file", RespondJSON(http.StatusOK, map[string]interface{}{"FileContent": "version: '3'\n"}))
	return mock
}

// TestAppTemplatesDataSource_ListAndFile: the catalogue is mapped and a
// template_id triggers the file lookup, which Portainer models as a POST.
func TestAppTemplatesDataSource_ListAndFile(t *testing.T) {
	t.Parallel()
	mock := appTemplatesMock(t)
	const ds = "data.portainer_app_templates.t"
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `data "portainer_app_templates" "t" { template_id = 12 }`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(ds, "id", "app-templates-3"),
				resource.TestCheckResourceAttr(ds, "version", "3"),
				resource.TestCheckResourceAttr(ds, "templates.#", "2"),
				resource.TestCheckResourceAttr(ds, "templates.0.title", "WordPress"),
				resource.TestCheckResourceAttr(ds, "templates.0.repository_url", "https://github.com/acme/templates"),
				resource.TestCheckResourceAttr(ds, "templates.0.categories.0", "cms"),
				resource.TestCheckResourceAttr(ds, "templates.1.categories.#", "0"),
				resource.TestCheckResourceAttr(ds, "templates.1.image", "nginx:latest"),
				resource.TestCheckResourceAttr(ds, "file_content", "version: '3'\n"),
			),
		}},
	})
}

// TestAppTemplatesDataSource_WithoutTemplateID: the file endpoint is left
// alone when no template is named.
func TestAppTemplatesDataSource_WithoutTemplateID(t *testing.T) {
	t.Parallel()
	mock := appTemplatesMock(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `data "portainer_app_templates" "t" {}`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("data.portainer_app_templates.t", "file_content", ""),
				func(*terraform.State) error {
					if mock.FindRequest("POST", "/templates/12/file") != nil {
						return fmt.Errorf("the file endpoint must not be called without template_id")
					}
					return nil
				},
			),
		}},
	})
}

// TestAccAppTemplatesDataSource lists the template catalogue of a real Portainer.
func TestAccAppTemplatesDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: accProvider() + `data "portainer_app_templates" "t" {}`,
			Check:  resource.TestCheckResourceAttrSet("data.portainer_app_templates.t", "templates.#"),
		}},
	})
}
