package internal

import (
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestHelmGitDryRunDataSource: the payload carries only the set arguments,
// the outputs and ID are read back and the boolean defaults are reported.
func TestHelmGitDryRunDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	rendered := "apiVersion: v1\nkind: Service\nmetadata:\n  name: app\n"
	mock.On("POST", "/endpoints/3/kubernetes/helm/git/dryrun", RespondJSON(http.StatusOK, map[string]interface{}{
		"manifest": rendered, "name": "my-release", "namespace": "default", "version": 2,
	}))
	const ds = "data.portainer_helm_git_dryrun.d"
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
data "portainer_helm_git_dryrun" "d" {
  endpoint_id    = 3
  repository_url = "https://github.com/owner/chart.git"
  reference_name = "refs/heads/main"
  chart_path     = "charts/myapp"
  values_files   = ["values.yaml", "values-prod.yaml"]
  namespace      = "default"
  release_name   = "my-release"
}
`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(ds, "id", "helm-git-dryrun-3-https://github.com/owner/chart.git"),
				resource.TestCheckResourceAttr(ds, "manifest", rendered),
				resource.TestCheckResourceAttr(ds, "release_version", "2"),
				resource.TestCheckResourceAttr(ds, "tls_skip_verify", "false"),
				resource.TestCheckResourceAttr(ds, "repository_authentication", "false"),
				func(*terraform.State) error {
					var payload map[string]interface{}
					post := mock.FindRequest("POST", "/endpoints/3/kubernetes/helm/git/dryrun")
					if post == nil || post.DecodeJSON(&payload) != nil {
						return fmt.Errorf("expected a JSON POST to the dry run endpoint")
					}
					want := map[string]interface{}{
						"repositoryURL": "https://github.com/owner/chart.git", "repositoryReferenceName": "refs/heads/main",
						"helmChartPath": "charts/myapp", "namespace": "default", "name": "my-release",
					}
					for k, v := range want {
						if payload[k] != v {
							return fmt.Errorf("payload.%s: got %v", k, payload[k])
						}
					}
					files, ok := payload["helmValuesFiles"].([]interface{})
					if !ok || len(files) != 2 || files[0] != "values.yaml" || files[1] != "values-prod.yaml" {
						return fmt.Errorf("payload.helmValuesFiles: got %v", payload["helmValuesFiles"])
					}
					for _, k := range []string{"repositoryAuthentication", "tlsSkipVerify", "repositoryGitCredentialID", "repositoryUsername"} {
						if _, set := payload[k]; set {
							return fmt.Errorf("payload.%s must be left out when unset", k)
						}
					}
					return nil
				},
			),
		}},
	})
}

func TestHelmGitDryRunDataSource_HTTPError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/endpoints/1/kubernetes/helm/git/dryrun", RespondString(http.StatusInternalServerError, "application/json", `{"message":"chart render failed"}`))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `data "portainer_helm_git_dryrun" "d" {` + "\n" + `endpoint_id = 1` + "\n" + `repository_url = "https://example.com/chart.git"` + "\n}",
			ExpectError: regexp.MustCompile(`failed to perform Helm Git dry run`),
		}},
	})
}
