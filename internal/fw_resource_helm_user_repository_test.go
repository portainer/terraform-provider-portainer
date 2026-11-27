package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

type helmRepoFakeEntry struct {
	ID     int    `json:"Id"`
	URL    string `json:"URL"`
	UserID int    `json:"UserId"`
}

// fakeHelmUserRepos is a stateful stand-in for /users/{id}/helm/repositories.
type fakeHelmUserRepos struct {
	mu     sync.Mutex
	nextID int
	repos  []helmRepoFakeEntry
}

func newFakeHelmUserRepos(m *MockServer, userID, maxID int) *fakeHelmUserRepos {
	f := &fakeHelmUserRepos{nextID: 10}
	base := fmt.Sprintf("/users/%d/helm/repositories", userID)
	m.On("GET", base, func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		RespondJSON(http.StatusOK, map[string]interface{}{"UserRepositories": append([]helmRepoFakeEntry{}, f.repos...)})(w, r)
	})
	m.On("POST", base, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			URL string `json:"url"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.nextID++
		e := helmRepoFakeEntry{ID: f.nextID, URL: body.URL, UserID: userID}
		f.repos = append(f.repos, e)
		RespondJSON(http.StatusOK, e)(w, r)
	})
	for id := 11; id <= maxID; id++ {
		id := id
		m.On("DELETE", fmt.Sprintf("%s/%d", base, id), func(w http.ResponseWriter, _ *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			for i, e := range f.repos {
				if e.ID == id {
					f.repos = append(f.repos[:i], f.repos[i+1:]...)
					break
				}
			}
			w.WriteHeader(http.StatusNoContent)
		})
	}
	return f
}

func helmUserRepoConfig(mock *MockServer, url string) string {
	return mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_helm_user_repository" "r" {
  user_id = 7
  url     = %q
}
`, url)
}

// TestHelmUserRepositoryResource_Lifecycle: create, a new URL replaces the
// repository, destroy.
func TestHelmUserRepositoryResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeHelmUserRepos(mock, 7, 13)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		CheckDestroy: func(*terraform.State) error {
			for _, p := range []string{"/users/7/helm/repositories/11", "/users/7/helm/repositories/12"} {
				if mock.FindRequest("DELETE", p) == nil {
					return fmt.Errorf("expected DELETE %s", p)
				}
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: helmUserRepoConfig(mock, "https://charts.bitnami.com/bitnami"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_helm_user_repository.r", "id", "11"),
					resource.TestCheckResourceAttr("portainer_helm_user_repository.r", "url", "https://charts.bitnami.com/bitnami"),
					func(*terraform.State) error {
						var payload map[string]interface{}
						post := mock.FindRequest("POST", "/users/7/helm/repositories")
						if post == nil || post.DecodeJSON(&payload) != nil || payload["url"] != "https://charts.bitnami.com/bitnami" {
							return fmt.Errorf("unexpected POST %+v", post)
						}
						return nil
					},
				),
			},
			{
				Config: helmUserRepoConfig(mock, "https://other.example.com"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_helm_user_repository.r", plancheck.ResourceActionDestroyBeforeCreate)},
				},
				Check: resource.TestCheckResourceAttr("portainer_helm_user_repository.r", "id", "12"),
			},
		},
	})
}

func TestHelmUserRepositoryResource_GoneOutsideTerraform(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeHelmUserRepos(mock, 7, 12)
	config := helmUserRepoConfig(mock, "https://charts.bitnami.com/bitnami")
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					fake.mu.Lock()
					fake.repos = nil
					fake.mu.Unlock()
				},
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestHelmUserRepositoryResource_Errors(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/users/7/helm/repositories", RespondString(http.StatusBadRequest, "application/json", `{"message":"bad url"}`))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config:      helmUserRepoConfig(mock, "ftp://charts.example.com"),
				ExpectError: regexp.MustCompile(`to have a url with schema of: "http,https"`),
			},
			{
				Config:      helmUserRepoConfig(mock, "https://charts.example.com"),
				ExpectError: regexp.MustCompile(`failed to create helm user repository`),
			},
		},
	})
}

func TestHelmUserRepositoryResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeHelmUserRepos(mock, 7, 11)
	config := helmUserRepoConfig(mock, "https://charts.bitnami.com/bitnami")
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		Steps: []resource.TestStep{
			{ExternalProviders: previousRelease, Config: config},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TestAccHelmUserRepositoryResource adds a Helm repository to the admin user
// (ID 1) of a real Portainer.
func TestAccHelmUserRepositoryResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: accProvider() + `
resource "portainer_helm_user_repository" "r" {
  user_id = 1
  url     = "https://charts.bitnami.com/bitnami"
}
`,
			Check: resource.TestCheckResourceAttrSet("portainer_helm_user_repository.r", "id"),
		}},
	})
}
