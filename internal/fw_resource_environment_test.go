package internal

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// fakeEnvironments is a stateful stand-in for Portainer's environment
// (endpoint) API: multipart create, inspect, list, update, delete and the TLS
// file uploads. Like Portainer it stores Agent and Edge Agent URLs without
// the scheme, and an edge key carrying the submitted URL.
type fakeEnvironments struct {
	mu      sync.Mutex
	nextID  int64
	envs    map[int64]*fakeEnvironment
	creates []map[string]string // multipart fields of every POST /endpoints
	files   [][]string          // multipart file fields of every POST /endpoints
	uploads []string            // "<kind>:<folder>:<content>" of every TLS upload
	edgeID  string              // EdgeID returned on create (EnforceEdgeID)
	// businessEdition makes an update fail as Business Edition does when a
	// true TLS skip flag asks it to delete a CA or client certificate file
	// the environment does not have (issue #165); CE only logs a warning.
	businessEdition bool
}

type fakeEnvironment struct {
	ID                 int64                     `json:"Id"`
	Name               string                    `json:"Name"`
	Type               int64                     `json:"Type"`
	GroupID            int64                     `json:"GroupId"`
	URL                string                    `json:"URL"`
	PublicURL          string                    `json:"PublicURL"`
	EdgeID             string                    `json:"EdgeID"`
	EdgeKey            string                    `json:"EdgeKey"`
	TagIds             []int64                   `json:"TagIds"`
	TLSConfig          *portainerTLSConfig       `json:"TLSConfig"`
	ContainerEngine    string                    `json:"ContainerEngine"`
	UserAccessPolicies map[string]map[string]int `json:"UserAccessPolicies"`
	TeamAccessPolicies map[string]map[string]int `json:"TeamAccessPolicies"`

	EdgeCheckinInterval int64                 `json:"EdgeCheckinInterval"`
	Edge                portainerEndpointEdge `json:"Edge"`
	// Like Portainer, the fake never returns the Azure credentials.
	azure [3]string // application ID, tenant ID, key
}

func newFakeEnvironments(m *MockServer) *fakeEnvironments {
	f := &fakeEnvironments{envs: map[int64]*fakeEnvironment{}}
	m.On("GET", "/endpoints", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		list := []*fakeEnvironment{}
		for id := int64(1); id <= f.nextID+20; id++ {
			if e, ok := f.envs[id]; ok {
				list = append(list, e)
			}
		}
		RespondJSON(http.StatusOK, list)(w, r)
	})
	m.On("POST", "/endpoints", f.create)
	for id := 1; id <= 20; id++ {
		p := fmt.Sprintf("/endpoints/%d", id)
		m.On("GET", p, f.inspect)
		m.On("PUT", p, f.update)
		m.On("DELETE", p, f.delete)
	}
	for _, kind := range []string{"ca", "cert", "key"} {
		m.On("POST", "/upload/tls/"+kind, f.upload)
	}
	return f
}

func (f *fakeEnvironments) idOf(r *http.Request) int64 {
	id, _ := strconv.ParseInt(r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:], 10, 64)
	return id
}

func (f *fakeEnvironments) mutate(id int64, fn func(e *fakeEnvironment)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn(f.envs[id])
}

func (f *fakeEnvironments) add(e *fakeEnvironment) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if e.ID > f.nextID {
		f.nextID = e.ID
	}
	f.envs[e.ID] = e
}

func (f *fakeEnvironments) lastCreate() map[string]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.creates) == 0 {
		return nil
	}
	return f.creates[len(f.creates)-1]
}

func (f *fakeEnvironments) uploadList() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.uploads...)
}

func fakeParseMultipart(r *http.Request) (map[string]string, map[string]string, error) {
	_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return nil, nil, err
	}
	fields, files := map[string]string{}, map[string]string{}
	mr := multipart.NewReader(r.Body, params["boundary"])
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			return fields, files, nil
		}
		if err != nil {
			return nil, nil, err
		}
		b, _ := io.ReadAll(part)
		if part.FileName() != "" {
			files[part.FormName()] = string(b)
		} else {
			fields[part.FormName()] = string(b)
		}
	}
}

func (f *fakeEnvironments) create(w http.ResponseWriter, r *http.Request) {
	fields, files, err := fakeParseMultipart(r)
	if err != nil {
		RespondString(http.StatusBadRequest, "text/plain", err.Error())(w, r)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.creates = append(f.creates, fields)
	var names []string
	for k := range files {
		names = append(names, k)
	}
	f.files = append(f.files, names)

	f.nextID++
	t, _ := strconv.ParseInt(fields["EndpointCreationType"], 10, 64)
	g, _ := strconv.ParseInt(fields["GroupID"], 10, 64)
	e := &fakeEnvironment{ID: f.nextID, Name: fields["Name"], Type: t, GroupID: g, URL: fields["URL"], ContainerEngine: fields["ContainerEngine"], TagIds: []int64{}}
	if t == 2 || t == 4 || t == 7 {
		e.URL = trimURLScheme(e.URL)
	}
	if t == 4 || t == 7 {
		tunnel := fields["EdgeTunnelServerAddress"]
		if tunnel == "" {
			tunnel = e.URL + ":8000"
		}
		e.EdgeKey = base64.RawStdEncoding.EncodeToString([]byte(fmt.Sprintf("%s|%s|fp|%d", fields["URL"], tunnel, e.ID)))
		e.EdgeID = f.edgeID
		e.EdgeCheckinInterval, _ = strconv.ParseInt(fields["EdgeCheckinInterval"], 10, 64)
		e.Edge.AsyncMode = fields["EdgeAsyncMode"] == "true"
		e.Edge.PingInterval, _ = strconv.ParseInt(fields["EdgePingInterval"], 10, 64)
		e.Edge.SnapshotInterval, _ = strconv.ParseInt(fields["EdgeSnapshotInterval"], 10, 64)
		e.Edge.CommandInterval, _ = strconv.ParseInt(fields["EdgeCommandInterval"], 10, 64)
	} else {
		e.PublicURL = fields["PublicURL"]
		if e.PublicURL == "" {
			e.PublicURL = trimURLScheme(fields["URL"])
		}
		tls := fields["TLS"] == "true"
		e.TLSConfig = &portainerTLSConfig{TLS: tls}
		if tls {
			e.TLSConfig.TLSSkipVerify = fields["TLSSkipVerify"] == "true"
			if fields["TLSSkipClientVerify"] == "false" {
				e.TLSConfig.TLSCert = "/data/tls/cert.pem"
			}
		}
	}
	if v := fields["TagIds"]; v != "" {
		_ = json.Unmarshal([]byte(v), &e.TagIds)
	}
	e.azure = [3]string{fields["AzureApplicationID"], fields["AzureTenantID"], fields["AzureAuthenticationKey"]}
	f.envs[e.ID] = e
	RespondJSON(http.StatusOK, e)(w, r)
}

func (f *fakeEnvironments) inspect(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.envs[f.idOf(r)]
	if !ok {
		RespondString(http.StatusNotFound, "application/json", `{"message":"not found"}`)(w, r)
		return
	}
	RespondJSON(http.StatusOK, e)(w, r)
}

func (f *fakeEnvironments) update(w http.ResponseWriter, r *http.Request) {
	var body environmentUpdateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		RespondString(http.StatusBadRequest, "text/plain", err.Error())(w, r)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.envs[f.idOf(r)]
	if !ok {
		RespondString(http.StatusNotFound, "application/json", `{"message":"not found"}`)(w, r)
		return
	}
	if body.Name != "" {
		e.Name = body.Name
	}
	if body.GroupID != 0 {
		e.GroupID = int64(body.GroupID)
	}
	if body.URL != nil {
		e.URL = *body.URL
		if e.Type == 2 || e.Type == 6 {
			e.URL = trimURLScheme(e.URL)
		}
	}
	if body.PublicURL != "" {
		e.PublicURL = body.PublicURL
	}
	if body.TLS != nil {
		// As Portainer does, a skip flag left out of the request keeps its
		// current value. A true flag deletes the CA file (tlsskipVerify) or the
		// client certificate and key (tlsskipClientVerify); with TLS on, the
		// CA file exists unless server verification is skipped already.
		prev := portainerTLSConfig{}
		if e.TLSConfig != nil {
			prev = *e.TLSConfig
		}
		if *body.TLS && f.businessEdition {
			if body.TLSSkipVerify != nil && *body.TLSSkipVerify && (!prev.TLS || prev.TLSSkipVerify) {
				RespondString(http.StatusInternalServerError, "application/json", `{"message":"Unable to remove TLS CA file from disk"}`)(w, r)
				return
			}
			if body.TLSSkipClientVerify != nil && *body.TLSSkipClientVerify && prev.TLSCert == "" {
				RespondString(http.StatusInternalServerError, "application/json", `{"message":"Unable to remove TLS cert file from disk"}`)(w, r)
				return
			}
		}
		e.TLSConfig = &portainerTLSConfig{TLS: *body.TLS}
		if *body.TLS {
			e.TLSConfig.TLSSkipVerify = prev.TLS && prev.TLSSkipVerify
			if body.TLSSkipVerify != nil {
				e.TLSConfig.TLSSkipVerify = *body.TLSSkipVerify
			}
			if prev.TLS {
				e.TLSConfig.TLSCert = prev.TLSCert
			}
			if body.TLSSkipClientVerify != nil {
				e.TLSConfig.TLSCert = ""
				if !*body.TLSSkipClientVerify {
					e.TLSConfig.TLSCert = "/data/tls/cert.pem"
				}
			}
		}
	}
	if body.TagIDs != nil {
		e.TagIds = []int64{}
		for _, t := range body.TagIDs {
			e.TagIds = append(e.TagIds, int64(t))
		}
	}
	for i, v := range []*string{body.AzureApplicationID, body.AzureTenantID, body.AzureAuthenticationKey} {
		if v != nil {
			e.azure[i] = *v
		}
	}
	if body.EdgeCheckinInterval != nil {
		e.EdgeCheckinInterval = *body.EdgeCheckinInterval
	}
	if body.Edge != nil {
		e.Edge.PingInterval, e.Edge.SnapshotInterval, e.Edge.CommandInterval = body.Edge.PingInterval, body.Edge.SnapshotInterval, body.Edge.CommandInterval
	}
	RespondJSON(http.StatusOK, e)(w, r)
}

func (f *fakeEnvironments) delete(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.idOf(r)
	if _, ok := f.envs[id]; !ok {
		RespondString(http.StatusNotFound, "application/json", `{"message":"not found"}`)(w, r)
		return
	}
	delete(f.envs, id)
	w.WriteHeader(http.StatusNoContent)
}

func (f *fakeEnvironments) upload(w http.ResponseWriter, r *http.Request) {
	fields, files, err := fakeParseMultipart(r)
	if err != nil {
		RespondString(http.StatusBadRequest, "text/plain", err.Error())(w, r)
		return
	}
	kind := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
	f.mu.Lock()
	f.uploads = append(f.uploads, kind+":"+fields["folder"]+":"+files["file"])
	f.mu.Unlock()
	RespondJSON(http.StatusOK, map[string]interface{}{})(w, r)
}

// lastPut decodes the last PUT /endpoints/{id} body (nil when none was sent).
func environmentLastPut(t *testing.T, mock *MockServer, id string) map[string]interface{} {
	t.Helper()
	var last *RecordedRequest
	for _, r := range mock.Requests() {
		if r.Method == http.MethodPut && r.Path == "/endpoints/"+id {
			last = r
		}
	}
	if last == nil {
		return nil
	}
	var payload map[string]interface{}
	if err := last.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode PUT body: %v", err)
	}
	return payload
}

func environmentCount(mock *MockServer, method, path string) int {
	n := 0
	for _, r := range mock.Requests() {
		if r.Method == method && r.Path == path {
			n++
		}
	}
	return n
}

func checkEnv(fn func() error) resource.TestCheckFunc {
	return func(*terraform.State) error { return fn() }
}

const envDocker = `
resource "portainer_environment" "e" {
  name                = "prod"
  environment_address = "tcp://docker.example:2375"
  type                = 1
}
`

// TestEnvironmentResource_DockerLifecycle: create (multipart fields, no edge
// ID for a Docker environment), an update of the name, tags and public URL
// (full PUT payload), clearing the tags, and import.
func TestEnvironmentResource_DockerLifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEnvironments(mock)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + envDocker,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_environment.e", "id", "1"),
					resource.TestCheckResourceAttr("portainer_environment.e", "edge_id", ""),
					resource.TestCheckResourceAttr("portainer_environment.e", "public_ip", "docker.example:2375"),
					resource.TestCheckResourceAttr("portainer_environment.e", "container_engine", "docker"),
					resource.TestCheckNoResourceAttr("portainer_environment.e", "tag_ids"),
					checkEnv(func() error {
						got := fake.lastCreate()
						want := map[string]string{"Name": "prod", "EndpointCreationType": "1", "URL": "tcp://docker.example:2375", "GroupID": "1", "TLS": "true", "TLSSkipVerify": "true", "TLSSkipClientVerify": "true"}
						for k, v := range want {
							if got[k] != v {
								return fmt.Errorf("create field %s: want %q, got %q (all: %v)", k, v, got[k], got)
							}
						}
						for _, k := range []string{"ContainerEngine", "TagIds", "PublicURL", "AzureApplicationID", "AzureTenantID", "AzureAuthenticationKey", "EdgeCheckinInterval", "EdgeTunnelServerAddress", "EdgeAsyncMode", "EdgePingInterval", "EdgeSnapshotInterval", "EdgeCommandInterval", "CustomTemplateID", "CustomTemplateContent", "StackName"} {
							if _, ok := got[k]; ok {
								return fmt.Errorf("create field %s must not be sent: %v", k, got)
							}
						}
						if environmentCount(mock, "PUT", "/endpoints/1") != 0 {
							return fmt.Errorf("a plain create must not update the environment")
						}
						return nil
					}),
				),
			},
			{
				Config: mockProviderConfig(mock) + `
resource "portainer_environment" "e" {
  name                = "prod-renamed"
  environment_address = "tcp://docker.example:2375"
  type                = 1
  group_id            = 2
  tag_ids             = [3, 4]
  public_ip           = "10.0.0.1"
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_environment.e", plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_environment.e", "tag_ids.#", "2"),
					resource.TestCheckResourceAttr("portainer_environment.e", "public_ip", "10.0.0.1"),
					checkEnv(func() error {
						p := environmentLastPut(t, mock, "1")
						if p == nil {
							return fmt.Errorf("expected PUT /endpoints/1")
						}
						if p["name"] != "prod-renamed" || p["groupID"] != float64(2) || p["url"] != "tcp://docker.example:2375" || p["publicURL"] != "10.0.0.1" {
							return fmt.Errorf("unexpected PUT payload %v", p)
						}
						if p["tls"] != true {
							return fmt.Errorf("tls must be sent for a directly connected environment: %v", p)
						}
						// Unchanged true skip flags are left out: Business Edition
						// fails when they ask it to delete a missing file (#165).
						for _, k := range []string{"tlsskipVerify", "tlsskipClientVerify"} {
							if _, ok := p[k]; ok {
								return fmt.Errorf("unchanged %s must not be sent: %v", k, p)
							}
						}
						if fmt.Sprint(p["tagIDs"]) != "[3 4]" {
							return fmt.Errorf("tagIDs: %v", p["tagIDs"])
						}
						if p["userAccessPolicies"] != nil || p["teamAccessPolicies"] != nil {
							return fmt.Errorf("untouched policies must stay null: %v", p)
						}
						for _, k := range []string{"AzureApplicationID", "AzureTenantID", "AzureAuthenticationKey", "EdgeCheckinInterval", "Edge"} {
							if _, ok := p[k]; ok {
								return fmt.Errorf("unset %s must not be sent: %v", k, p)
							}
						}
						return nil
					}),
				),
			},
			{
				// Removing every tag sends an empty list.
				Config: mockProviderConfig(mock) + `
resource "portainer_environment" "e" {
  name                = "prod-renamed"
  environment_address = "tcp://docker.example:2375"
  type                = 1
  group_id            = 2
  public_ip           = "10.0.0.1"
}
`,
				Check: checkEnv(func() error {
					p := environmentLastPut(t, mock, "1")
					if tags, ok := p["tagIDs"].([]interface{}); !ok || len(tags) != 0 {
						return fmt.Errorf("expected tagIDs [] to clear the tags, got %v", p["tagIDs"])
					}
					return nil
				}),
			},
			{
				ResourceName:            "portainer_environment.e",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"tag_ids"},
			},
		},
	})
}

// TestEnvironmentResource_PublicURLFollowsAddress: without public_ip, a new
// address of a directly connected environment becomes its public URL too.
func TestEnvironmentResource_PublicURLFollowsAddress(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeEnvironments(mock)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: mockProviderConfig(mock) + envDocker},
			{
				Config: mockProviderConfig(mock) + strings.Replace(envDocker, "docker.example:2375", "docker2.example:2375", 1),
				Check: resource.ComposeTestCheckFunc(
					checkEnv(func() error {
						if p := environmentLastPut(t, mock, "1"); p["publicURL"] != "tcp://docker2.example:2375" {
							return fmt.Errorf("publicURL: %v", p["publicURL"])
						}
						return nil
					}),
				),
			},
			{
				// The next refresh picks the new public URL up; nothing to do.
				Config:   mockProviderConfig(mock) + strings.Replace(envDocker, "docker.example:2375", "docker2.example:2375", 1),
				PlanOnly: true,
			},
		},
	})
}

// TestEnvironmentResource_EdgeAgent: a type 4 create sends ContainerEngine and
// no TLS fields, generates an edge ID when Portainer returns none, applies
// tags and the public URL with a follow-up update that leaves the URL and TLS
// out, reads the address back from the edge key, ignores Portainer's
// conversion to type 7, and replaces the environment for a real address or
// engine change but not for a scheme-only one.
func TestEnvironmentResource_EdgeAgent(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEnvironments(mock)
	edge := func(addr, extra string) string {
		return mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_environment" "e" {
  name                = "edge-prod"
  environment_address = %q
  type                = 4
  %s
}
`, addr, extra)
	}
	var firstEdgeID string

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: edge("http://portainer:9000", `tag_ids = [5]
  public_ip = "edge.example.com"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_environment.e", "environment_address", "http://portainer:9000"),
					resource.TestCheckResourceAttrSet("portainer_environment.e", "edge_key"),
					resource.TestMatchResourceAttr("portainer_environment.e", "edge_id", regexp.MustCompile(`^[0-9a-f-]{36}$`)),
					resource.TestCheckResourceAttr("portainer_environment.e", "container_engine", "docker"),
					func(s *terraform.State) error {
						firstEdgeID = s.RootModule().Resources["portainer_environment.e"].Primary.Attributes["edge_id"]
						return nil
					},
					checkEnv(func() error {
						got := fake.lastCreate()
						if got["EndpointCreationType"] != "4" || got["ContainerEngine"] != "docker" {
							return fmt.Errorf("edge create fields: %v", got)
						}
						for _, k := range []string{"TLS", "TLSSkipVerify", "TLSSkipClientVerify", "TagIds"} {
							if _, ok := got[k]; ok {
								return fmt.Errorf("edge create must not send %s: %v", k, got)
							}
						}
						p := environmentLastPut(t, mock, "1")
						if p == nil {
							return fmt.Errorf("expected a follow-up PUT for the tags and the public URL")
						}
						for _, k := range []string{"url", "tls", "tlsskipVerify", "tlsskipClientVerify"} {
							if _, ok := p[k]; ok {
								return fmt.Errorf("edge update must not send %s: %v", k, p)
							}
						}
						if p["name"] != "edge-prod" || p["groupID"] != float64(1) || p["publicURL"] != "edge.example.com" || fmt.Sprint(p["tagIDs"]) != "[5]" {
							return fmt.Errorf("edge update payload: %v", p)
						}
						return nil
					}),
				),
			},
			{
				// The agent connected on Kubernetes: Portainer reports type 7.
				PreConfig: func() { fake.mutate(1, func(e *fakeEnvironment) { e.Type = 7 }) },
				Config: edge("http://portainer:9000", `tag_ids = [5]
  public_ip = "edge.example.com"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_environment.e", "type", "7"),
					func(s *terraform.State) error {
						if got := s.RootModule().Resources["portainer_environment.e"].Primary.Attributes["edge_id"]; got != firstEdgeID {
							return fmt.Errorf("edge_id changed on refresh: %q -> %q", firstEdgeID, got)
						}
						return nil
					},
				),
			},
			{
				PreConfig: func() { fake.mutate(1, func(e *fakeEnvironment) { e.Type = 4 }) },
				Config: edge("http://portainer2:9000", `tag_ids = [5]
  public_ip = "edge.example.com"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("portainer_environment.e", plancheck.ResourceActionDestroyBeforeCreate),
				}},
				Check: resource.TestCheckResourceAttr("portainer_environment.e", "id", "2"),
			},
			{
				Config: edge("http://portainer2:9000", `tag_ids = [5]
  public_ip = "edge.example.com"
  container_engine = "podman"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("portainer_environment.e", plancheck.ResourceActionDestroyBeforeCreate),
				}},
				Check: checkEnv(func() error {
					if got := fake.lastCreate(); got["ContainerEngine"] != "podman" {
						return fmt.Errorf("expected ContainerEngine=podman, got %v", got)
					}
					return nil
				}),
			},
			{
				// An older Portainer stores an empty engine for Docker; with the
				// engine omitted that must not replace anything.
				PreConfig: func() { fake.mutate(3, func(e *fakeEnvironment) { e.ContainerEngine = "" }) },
				Config: edge("http://portainer2:9000", `tag_ids = [5]
  public_ip = "edge.example.com"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("portainer_environment.e", plancheck.ResourceActionNoop),
				}},
			},
			{
				// A scheme-only difference is not a move.
				Config: edge("https://portainer2:9000", `tag_ids = [5]
  public_ip = "edge.example.com"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("portainer_environment.e", plancheck.ResourceActionUpdate),
				}},
				// The edge key keeps the address the environment was created
				// with, as with SDKv2.
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestEnvironmentResource_EdgeAgentServerEdgeID: an edge ID Portainer returns
// (EnforceEdgeID) is used as is.
func TestEnvironmentResource_EdgeAgentServerEdgeID(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEnvironments(mock)
	fake.edgeID = "server-edge-id"
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
resource "portainer_environment" "e" {
  name                = "edge"
  environment_address = "https://portainer.example.com"
  type                = 7
}
`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("portainer_environment.e", "edge_id", "server-edge-id"),
				checkEnv(func() error {
					if got := fake.lastCreate(); got["EndpointCreationType"] != "7" || got["ContainerEngine"] != "" {
						return fmt.Errorf("type 7 create must leave the engine out: %v", got)
					}
					return nil
				}),
			),
		}},
	})
}

// TestEnvironmentResource_AgentKeepsScheme: Portainer drops the scheme of an
// Agent address; the configured one stays and the next plan is empty. Type 6
// is created as an Agent (2) and Portainer's 2 -> 6 conversion is ignored.
func TestEnvironmentResource_AgentKeepsScheme(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEnvironments(mock)
	cfg := func(typ int) string {
		return mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_environment" "e" {
  name                = "agent"
  environment_address = "tcp://agent.example:9001"
  type                = %d
}
`, typ)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: cfg(6),
				Check: checkEnv(func() error {
					if got := fake.lastCreate(); got["EndpointCreationType"] != "2" {
						return fmt.Errorf("type 6 must be created as 2: %v", got)
					}
					return nil
				}),
				// Portainer reported type 2 for it.
				ExpectNonEmptyPlan: true,
			},
			{Config: cfg(2), PlanOnly: true},
			{
				PreConfig: func() { fake.mutate(1, func(e *fakeEnvironment) { e.Type = 6 }) },
				Config:    cfg(2),
				PlanOnly:  true,
			},
		},
	})
}

// TestEnvironmentResource_ExistingName: an environment of the same name is
// adopted and updated instead of created.
func TestEnvironmentResource_ExistingName(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEnvironments(mock)
	fake.add(&fakeEnvironment{ID: 7, Name: "prod", Type: 1, GroupID: 1, URL: "tcp://old:2375", TLSConfig: &portainerTLSConfig{TLS: true, TLSSkipVerify: true}})
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + envDocker,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("portainer_environment.e", "id", "7"),
				checkEnv(func() error {
					if mock.FindRequest("POST", "/endpoints") != nil {
						return fmt.Errorf("an existing environment must not be created again")
					}
					p := environmentLastPut(t, mock, "7")
					if p == nil || p["url"] != "tcp://docker.example:2375" || p["publicURL"] != "tcp://docker.example:2375" {
						return fmt.Errorf("expected the existing environment to be updated: %v", p)
					}
					return nil
				}),
			),
		}},
	})
}

// TestEnvironmentResource_AccessPolicies: policies are applied by an update
// after create, and only sent while configured or changed.
func TestEnvironmentResource_AccessPolicies(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeEnvironments(mock)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + `
resource "portainer_environment" "e" {
  name                 = "prod"
  environment_address  = "tcp://docker.example:2375"
  type                 = 1
  user_access_policies = { "2" = 1 }
  team_access_policies = { "3" = 2 }
}
`,
				Check: checkEnv(func() error {
					if n := environmentCount(mock, "PUT", "/endpoints/1"); n != 2 {
						return fmt.Errorf("expected one update per policy map, as before, got %d", n)
					}
					p := environmentLastPut(t, mock, "1")
					if fmt.Sprint(p["userAccessPolicies"]) != "map[2:map[RoleId:1]]" || fmt.Sprint(p["teamAccessPolicies"]) != "map[3:map[RoleId:2]]" {
						return fmt.Errorf("policies: %v", p)
					}
					return nil
				}),
			},
			{
				// Removing the team policies clears them.
				Config: mockProviderConfig(mock) + `
resource "portainer_environment" "e" {
  name                 = "prod"
  environment_address  = "tcp://docker.example:2375"
  type                 = 1
  user_access_policies = { "2" = 1 }
}
`,
				Check: checkEnv(func() error {
					p := environmentLastPut(t, mock, "1")
					if m, ok := p["teamAccessPolicies"].(map[string]interface{}); !ok || len(m) != 0 {
						return fmt.Errorf("expected teamAccessPolicies {} to clear them, got %v", p["teamAccessPolicies"])
					}
					return nil
				}),
			},
		},
	})
}

const envTLS = `
resource "portainer_environment" "e" {
  name                   = %q
  environment_address    = "tcp://docker.example:2376"
  type                   = 1
  tls_skip_verify        = false
  tls_skip_client_verify = false
  tls_ca_cert            = "CA"
  tls_cert               = %q
  tls_key                = "KEY"
}
`

// TestEnvironmentResource_TLS: create sends the certificates as files; a
// rotated certificate is uploaded before the update; an update without TLS
// change uploads nothing; switching TLS off reaches Portainer; a changed TLS
// configuration in Portainer shows as drift.
func TestEnvironmentResource_TLS(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEnvironments(mock)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(envTLS, "prod", "CERT"),
				Check: checkEnv(func() error {
					fake.mu.Lock()
					files := strings.Join(fake.files[0], ",")
					fake.mu.Unlock()
					for _, f := range []string{"TLSCACertFile", "TLSCertFile", "TLSKeyFile"} {
						if !strings.Contains(files, f) {
							return fmt.Errorf("expected %s in the create request, got %s", f, files)
						}
					}
					if got := fake.lastCreate(); got["TLSSkipVerify"] != "false" || got["TLSSkipClientVerify"] != "false" {
						return fmt.Errorf("TLS flags: %v", got)
					}
					return nil
				}),
			},
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(envTLS, "prod", "NEW-CERT"),
				Check: checkEnv(func() error {
					if got := strings.Join(fake.uploadList(), ";"); got != "ca:1:CA;cert:1:NEW-CERT;key:1:KEY" {
						return fmt.Errorf("uploads: %s", got)
					}
					// false skip flags point Portainer at the uploaded files and
					// reload the proxy, so they are sent on every update.
					p := environmentLastPut(t, mock, "1")
					if p["tls"] != true || p["tlsskipVerify"] != false || p["tlsskipClientVerify"] != false {
						return fmt.Errorf("a certificate rotation must send tls and the false skip flags: %v", p)
					}
					return nil
				}),
			},
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(envTLS, "renamed", "NEW-CERT"),
				Check: checkEnv(func() error {
					if n := len(fake.uploadList()); n != 3 {
						return fmt.Errorf("an update without TLS change must not upload, got %d uploads", n)
					}
					return nil
				}),
			},
			{
				// TLS switched off in the Portainer UI shows as drift.
				PreConfig: func() { fake.mutate(1, func(e *fakeEnvironment) { e.TLSConfig = &portainerTLSConfig{TLS: false} }) },
				Config:    mockProviderConfig(mock) + fmt.Sprintf(envTLS, "renamed", "NEW-CERT"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("portainer_environment.e", plancheck.ResourceActionUpdate),
				}},
			},
			{
				Config: mockProviderConfig(mock) + `
resource "portainer_environment" "e" {
  name                = "renamed"
  environment_address = "tcp://docker.example:2376"
  type                = 1
  tls_enabled         = false
}
`,
				Check: checkEnv(func() error {
					p := environmentLastPut(t, mock, "1")
					if p["tls"] != false {
						return fmt.Errorf("tls=false must be sent, got %v", p)
					}
					return nil
				}),
			},
		},
	})
}

// TestEnvironmentResource_TLSUploadFailureStopsUpdate: no PUT after a failed
// upload.
func TestEnvironmentResource_TLSUploadFailureStopsUpdate(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/upload/tls/cert", RespondString(http.StatusInternalServerError, "application/json", `{"message":"disk full"}`))
	newFakeEnvironments(mock)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: mockProviderConfig(mock) + fmt.Sprintf(envTLS, "prod", "CERT")},
			{
				Config:      mockProviderConfig(mock) + fmt.Sprintf(envTLS, "prod", "NEW-CERT"),
				ExpectError: regexp.MustCompile(`failed to upload TLS files for environment`),
			},
			{
				Config:             mockProviderConfig(mock) + fmt.Sprintf(envTLS, "prod", "CERT"),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
				PreConfig: func() {
					if environmentCount(mock, "PUT", "/endpoints/1") != 0 {
						t.Error("expected no PUT after the failed upload")
					}
				},
			},
		},
	})
}

// TestEnvironmentResource_WriteOnlyKey: tls_key_wo is uploaded on create and
// when its version changes, and never stored.
func TestEnvironmentResource_WriteOnlyKey(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEnvironments(mock)
	cfg := func(key string, version int) string {
		return mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_environment" "e" {
  name                   = "prod"
  environment_address    = "tcp://docker.example:2376"
  type                   = 1
  tls_skip_client_verify = false
  tls_cert               = "CERT"
  tls_key_wo             = %q
  tls_key_wo_version     = %d
}
`, key, version)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: cfg("WO-KEY", 1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("portainer_environment.e", "tls_key_wo"),
					resource.TestCheckNoResourceAttr("portainer_environment.e", "tls_key"),
					checkEnv(func() error {
						fake.mu.Lock()
						defer fake.mu.Unlock()
						if !strings.Contains(strings.Join(fake.files[0], ","), "TLSKeyFile") {
							return fmt.Errorf("expected the write-only key in the create request: %v", fake.files[0])
						}
						return nil
					}),
				),
			},
			{Config: cfg("WO-KEY-2", 1), PlanOnly: true},
			{
				Config: cfg("WO-KEY-2", 2),
				Check: checkEnv(func() error {
					if got := strings.Join(fake.uploadList(), ";"); got != "cert:1:CERT;key:1:WO-KEY-2" {
						return fmt.Errorf("uploads: %s", got)
					}
					return nil
				}),
			},
		},
	})
}

func TestEnvironmentResource_ConflictingKeys(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
resource "portainer_environment" "e" {
  name                = "prod"
  environment_address = "tcp://docker.example:2376"
  type                = 1
  tls_key             = "KEY"
  tls_key_wo          = "KEY"
  tls_key_wo_version  = 1
}
`,
			ExpectError: regexp.MustCompile(`(?s)tls_key.*cannot be specified`),
		}},
	})
}

func TestEnvironmentResource_TypeValidation(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + strings.Replace(envDocker, "type                = 1", "type                = 9", 1),
			ExpectError: regexp.MustCompile(`between 1 and 7`),
		}},
	})
}

func TestEnvironmentResource_CreateErrors(t *testing.T) {
	t.Parallel()
	t.Run("list", func(t *testing.T) {
		mock := NewMockServer(t)
		mock.On("GET", "/endpoints", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))
		resource.UnitTest(t, resource.TestCase{
			ProtoV5ProviderFactories: testProtoV5Factories,
			Steps:                    []resource.TestStep{{Config: mockProviderConfig(mock) + envDocker, ExpectError: regexp.MustCompile(`failed to check for existing environment`)}},
		})
	})
	t.Run("create", func(t *testing.T) {
		mock := NewMockServer(t)
		mock.On("POST", "/endpoints", RespondString(http.StatusBadRequest, "application/json", `{"message":"Invalid request payload"}`))
		newFakeEnvironments(mock)
		resource.UnitTest(t, resource.TestCase{
			ProtoV5ProviderFactories: testProtoV5Factories,
			Steps:                    []resource.TestStep{{Config: mockProviderConfig(mock) + envDocker, ExpectError: regexp.MustCompile(`failed to create environment`)}},
		})
	})
}

// TestEnvironmentResource_GoneOutsideTerraform: an environment deleted in
// Portainer is planned for creation again.
func TestEnvironmentResource_GoneOutsideTerraform(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEnvironments(mock)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: mockProviderConfig(mock) + envDocker},
			{
				PreConfig:          func() { fake.mu.Lock(); delete(fake.envs, 1); fake.mu.Unlock() },
				Config:             mockProviderConfig(mock) + envDocker,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func shortenEnvironmentRemovePoll(t *testing.T) {
	t.Helper()
	old := environmentRemovePollInterval
	environmentRemovePollInterval = time.Millisecond
	t.Cleanup(func() { environmentRemovePollInterval = old })
}

// TestEnvironmentResource_DeleteCluster: delete_cluster_on_destroy alone
// changes nothing in Portainer; destroy then sends deleteCluster=true and
// waits for the environment to disappear.
func TestEnvironmentResource_DeleteCluster(t *testing.T) {
	shortenEnvironmentRemovePoll(t)
	mock := NewMockServer(t)
	var gets int32
	var deleted atomic.Bool
	mock.On("DELETE", "/endpoints/1", func(w http.ResponseWriter, r *http.Request) {
		deleted.Store(true)
		w.WriteHeader(http.StatusAccepted)
	})
	mock.On("GET", "/endpoints/1", func(w http.ResponseWriter, r *http.Request) {
		if !deleted.Load() {
			RespondJSON(http.StatusOK, map[string]interface{}{"Id": 1, "Name": "prod", "Type": 1, "GroupId": 1, "URL": "tcp://docker.example:2375"})(w, r)
			return
		}
		if atomic.AddInt32(&gets, 1) < 3 {
			RespondJSON(http.StatusOK, map[string]interface{}{"Id": 1, "Status": 3})(w, r)
			return
		}
		RespondString(http.StatusNotFound, "text/plain", "not found")(w, r)
	})
	newFakeEnvironments(mock)
	withFlag := strings.Replace(envDocker, "type                = 1", "type                = 1\n  delete_cluster_on_destroy = true", 1)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		CheckDestroy: func(*terraform.State) error {
			var req *RecordedRequest
			for _, r := range mock.Requests() {
				if r.Method == http.MethodDelete {
					req = r
				}
			}
			if req == nil || req.Query != "deleteCluster=true" {
				return fmt.Errorf("expected DELETE with deleteCluster=true, got %+v", req)
			}
			if n := atomic.LoadInt32(&gets); n != 3 {
				return fmt.Errorf("expected 3 GET polls, got %d", n)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{Config: mockProviderConfig(mock) + envDocker},
			{
				Config: mockProviderConfig(mock) + withFlag,
				Check: checkEnv(func() error {
					if environmentCount(mock, "PUT", "/endpoints/1") != 0 {
						return fmt.Errorf("expected no PUT when only delete_cluster_on_destroy changes")
					}
					return nil
				}),
			},
		},
	})
}

// TestEnvironmentResource_DeleteDefault: no query and no polling by default,
// and an environment already gone is fine.
func TestEnvironmentResource_DeleteDefault(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeEnvironments(mock)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		CheckDestroy: func(*terraform.State) error {
			req := mock.FindRequest("DELETE", "/endpoints/1")
			if req == nil || req.Query != "" {
				return fmt.Errorf("expected DELETE /endpoints/1 without query, got %+v", req)
			}
			for _, r := range mock.Requests() {
				if r.Method == http.MethodGet && r.Query == "excludeSnapshot=true" {
					return fmt.Errorf("expected no polling without delete_cluster_on_destroy")
				}
			}
			return nil
		},
		Steps: []resource.TestStep{{Config: mockProviderConfig(mock) + envDocker}},
	})
}

func TestEnvironmentResource_DeleteClusterConflict(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	var conflict atomic.Bool
	conflict.Store(true)
	fake := newFakeEnvironments(mock)
	mock.mu.Lock()
	// Put the conflicting DELETE in front of the fake's route.
	mock.routes = append([]route{{method: "DELETE", path: "/endpoints/1", handler: func(w http.ResponseWriter, r *http.Request) {
		if conflict.Load() {
			RespondString(http.StatusConflict, "application/json", `{"message":"installing"}`)(w, r)
			return
		}
		fake.delete(w, r)
	}}}, mock.routes...)
	mock.mu.Unlock()
	cfg := mockProviderConfig(mock) + strings.Replace(envDocker, "type                = 1", "type                = 1\n  delete_cluster_on_destroy = true", 1)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{Config: cfg, Destroy: true, ExpectError: regexp.MustCompile(`KubeSolo is still being installed`)},
			{Config: cfg, PreConfig: func() { conflict.Store(false) }},
		},
	})
}

func TestWaitEnvironmentRemoved_Timeout(t *testing.T) {
	shortenEnvironmentRemovePoll(t)
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/7", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 7}))

	err := waitEnvironmentRemoved(context.Background(), mock.Client(), 7, 20*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "still exists") {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

// TestEdgeAddressFromKey pins the edge key parsing used to recover
// environment_address: field 0 of "url|tunnel|fingerprint|id", accepting both
// the padded and the unpadded base64 Portainer emits.
func TestEdgeAddressFromKey(t *testing.T) {
	t.Parallel()
	key := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	rawKey := func(s string) string { return base64.RawStdEncoding.EncodeToString([]byte(s)) }

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"internal address with port", key("http://portainer:9000|portainer:8000|fp|21"), "http://portainer:9000"},
		{"public address", key("https://portainer.example.com|portainer.example.com:8000|fp|4"), "https://portainer.example.com"},
		{"unpadded key", rawKey("http://portainer:9000|portainer:8000|fp|21"), "http://portainer:9000"},
		{"surrounding spaces", key(" https://portainer.example.com |host:8000|fp|7"), "https://portainer.example.com"},
		{"too few fields", key("https://portainer.example.com|host:8000"), ""},
		{"not base64", "not-a-key", ""},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		if got := edgeAddressFromKey(tc.in); got != tc.want {
			t.Errorf("%s: edgeAddressFromKey(%q): expected %q, got %q", tc.name, tc.in, tc.want, got)
		}
	}
	if got := edgeTunnelFromKey(rawKey("http://portainer:9000|tunnel.example:8000|fp|21")); got != "tunnel.example:8000" {
		t.Errorf("edgeTunnelFromKey: got %q", got)
	}
	if got := edgeTunnelFromKey("not-a-key"); got != "" {
		t.Errorf("edgeTunnelFromKey of garbage: got %q", got)
	}
}

// TestEnvironmentPlanModifiers pins the suppressions that replaced the SDKv2
// DiffSuppressFuncs: a scheme-less address in the state against the same
// address with a scheme, and the 4 -> 7 / 2 -> 6 type conversions.
func TestEnvironmentPlanModifiers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	addr := func(state, config string) string {
		req := planmodifier.StringRequest{StateValue: types.StringValue(state), ConfigValue: types.StringValue(config), PlanValue: types.StringValue(config)}
		resp := &planmodifier.StringResponse{PlanValue: req.PlanValue}
		environmentAddressSuppress{}.PlanModifyString(ctx, req, resp)
		return resp.PlanValue.ValueString()
	}
	for _, tc := range []struct{ state, config, want string }{
		{"host:9001", "tcp://host:9001", "host:9001"},
		{"portainer.example.com", "https://portainer.example.com", "portainer.example.com"},
		{"tcp://host:9001", "tcp://other:9001", "tcp://other:9001"},
		{"host:9001", "tcp://other:9001", "tcp://other:9001"},
		{"https://a", "http://a", "http://a"},
	} {
		if got := addr(tc.state, tc.config); got != tc.want {
			t.Errorf("address %q -> %q: planned %q, want %q", tc.state, tc.config, got, tc.want)
		}
	}
	typ := func(state, config int64) int64 {
		req := planmodifier.Int64Request{StateValue: types.Int64Value(state), ConfigValue: types.Int64Value(config), PlanValue: types.Int64Value(config)}
		resp := &planmodifier.Int64Response{PlanValue: req.PlanValue}
		environmentTypeSuppress{}.PlanModifyInt64(ctx, req, resp)
		return resp.PlanValue.ValueInt64()
	}
	for _, tc := range []struct{ state, config, want int64 }{
		{7, 4, 7}, {6, 2, 6}, {4, 7, 7}, {2, 6, 6}, {1, 2, 2}, {5, 4, 4},
	} {
		if got := typ(tc.state, tc.config); got != tc.want {
			t.Errorf("type %d -> %d: planned %d, want %d", tc.state, tc.config, got, tc.want)
		}
	}
}

// TestEnvironmentResource_UpgradeFromV300: state written by v3.0.0 (SDKv2)
// against the same fake plans no change with this build, for configurations
// that set and that omit the optional attributes.
func TestEnvironmentResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	configs := map[string]string{
		"docker-minimal": envDocker,
		"docker-full": `
resource "portainer_environment" "e" {
  name                      = "prod"
  environment_address       = "tcp://docker.example:2375"
  type                      = 1
  group_id                  = 1
  tag_ids                   = [1, 2]
  public_ip                 = "10.0.0.1"
  tls_enabled               = true
  tls_skip_verify           = false
  tls_skip_client_verify    = false
  tls_ca_cert               = "CA"
  tls_cert                  = "CERT"
  tls_key                   = "KEY"
  user_access_policies      = { "2" = 1 }
  team_access_policies      = { "3" = 2 }
  delete_cluster_on_destroy = true
  timeouts { delete = "5m" }
}
`,
		"tls-off-empty-tags": `
resource "portainer_environment" "e" {
  name                = "prod"
  environment_address = "tcp://docker.example:2375"
  type                = 1
  tls_enabled         = false
  tag_ids             = []
}
`,
		"agent": `
resource "portainer_environment" "e" {
  name                = "agent"
  environment_address = "tcp://agent.example:9001"
  type                = 2
}
`,
		"edge-agent": `
resource "portainer_environment" "e" {
  name                = "edge"
  environment_address = "https://portainer.example.com"
  type                = 4
  tag_ids             = [5]
}
`,
		"edge-podman": `
resource "portainer_environment" "e" {
  name                = "edge"
  environment_address = "http://portainer:9000"
  type                = 4
  container_engine    = "podman"
}
`,
	}
	for name, cfg := range configs {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			newFakeEnvironments(mock)
			config := mockProviderConfig(mock) + cfg
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
		})
	}
}

// TestAccEnvironmentResource_EdgeAgent runs against a real Portainer: an Edge
// Agent environment needs no reachable agent to be created.
func TestAccEnvironmentResource_EdgeAgent(t *testing.T) {
	name := acctest.RandomWithPrefix("acc-env")
	config := fmt.Sprintf(`
resource "portainer_environment" "e" {
  name                = %q
  environment_address = "https://portainer.example.com"
  type                = 4
}

data "portainer_environment" "e" {
  name       = portainer_environment.e.name
  depends_on = [portainer_environment.e]
}
`, name)
	resource.Test(t, resource.TestCase{
		PreCheck: func() { accPreCheck(t) },
		Steps: []resource.TestStep{
			{
				ExternalProviders: previousRelease,
				Config:            accProvider() + config,
			},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   accProvider() + config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.portainer_environment.e", "id", "portainer_environment.e", "id"),
					resource.TestCheckResourceAttr("data.portainer_environment.e", "type", "4"),
					resource.TestCheckResourceAttrSet("portainer_environment.e", "edge_key"),
				),
			},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   accProvider() + config,
				ResourceName:             "portainer_environment.e",
				ImportState:              true,
				ImportStateVerify:        true,
				ImportStateVerifyIgnore:  []string{"tls_enabled", "tls_skip_verify", "tls_skip_client_verify", "edge_id", "timeouts"},
			},
		},
	})
}

func mockServerEdition(mock *MockServer, edition string) {
	mock.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]string{"ServerVersion": "2.45.3", "ServerEdition": edition}))
}

// TestEnvironmentResource_EdgeSettings: the edge agent settings of a Business
// Edition server go into the create request, are read back (unset ones as
// Portainer's value), changed intervals are sent with PUT /endpoints/{id}
// (the async ones as one object), and a new async mode or tunnel server
// address replaces the environment.
func TestEnvironmentResource_EdgeSettings(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mockServerEdition(mock, "EE")
	fake := newFakeEnvironments(mock)
	cfg := func(extra string) string {
		return mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_environment" "e" {
  name                = "edge"
  environment_address = "https://portainer.example.com"
  type                = 4
  %s
}
`, extra)
	}
	base := `edge_tunnel_server_address = "tunnel.example:8000"
  edge_async_mode            = true
  edge_checkin_interval      = 10
  edge_ping_interval         = 30
  edge_snapshot_interval     = 60
`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: cfg(base),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_environment.e", "edge_command_interval", "0"),
					resource.TestCheckResourceAttr("portainer_environment.e", "edge_ping_interval", "30"),
					resource.TestCheckResourceAttr("portainer_environment.e", "edge_async_mode", "true"),
					checkEnv(func() error {
						got := fake.lastCreate()
						want := map[string]string{"EdgeTunnelServerAddress": "tunnel.example:8000", "EdgeAsyncMode": "true", "EdgeCheckinInterval": "10", "EdgePingInterval": "30", "EdgeSnapshotInterval": "60"}
						for k, v := range want {
							if got[k] != v {
								return fmt.Errorf("create field %s: want %q, got %q", k, v, got[k])
							}
						}
						if _, ok := got["EdgeCommandInterval"]; ok {
							return fmt.Errorf("unset EdgeCommandInterval must not be sent: %v", got)
						}
						if environmentCount(mock, "PUT", "/endpoints/1") != 0 {
							return fmt.Errorf("a create without tags must not update the environment")
						}
						return nil
					}),
				),
			},
			{Config: cfg(base), PlanOnly: true},
			{
				// Set outside Terraform: an unset interval reports Portainer's value.
				PreConfig: func() { fake.mutate(1, func(e *fakeEnvironment) { e.Edge.CommandInterval = 15 }) },
				Config:    cfg(strings.Replace(strings.Replace(base, "= 30", "= 45", 1), "= 10", "= 20", 1)),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("portainer_environment.e", plancheck.ResourceActionUpdate),
				}},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_environment.e", "edge_command_interval", "15"),
					checkEnv(func() error {
						p := environmentLastPut(t, mock, "1")
						if p["EdgeCheckinInterval"] != float64(20) {
							return fmt.Errorf("EdgeCheckinInterval: %v", p)
						}
						if fmt.Sprint(p["Edge"]) != "map[CommandInterval:15 PingInterval:45 SnapshotInterval:60]" {
							return fmt.Errorf("Edge: %v", p["Edge"])
						}
						if _, ok := p["url"]; ok {
							return fmt.Errorf("edge update must not send the URL: %v", p)
						}
						return nil
					}),
				),
			},
			{
				Config:           cfg(strings.Replace(strings.Replace(base, "= 30", "= 45", 1), "= 10", "= 20", 1)),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				Config: cfg(strings.Replace(base, "tunnel.example:8000", "tunnel2.example:8000", 1)),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("portainer_environment.e", plancheck.ResourceActionDestroyBeforeCreate),
				}},
				Check: resource.TestCheckResourceAttr("portainer_environment.e", "id", "2"),
			},
			{
				Config: cfg(strings.Replace(strings.Replace(base, "tunnel.example:8000", "tunnel2.example:8000", 1), "true", "false", 1)),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("portainer_environment.e", plancheck.ResourceActionDestroyBeforeCreate),
				}},
				Check: checkEnv(func() error {
					if got := fake.lastCreate(); got["EdgeAsyncMode"] != "false" {
						return fmt.Errorf("EdgeAsyncMode: %v", got)
					}
					return nil
				}),
			},
		},
	})
}

// TestEnvironmentResource_EdgeTunnelAdopted: adding the tunnel server address
// the edge key already carries changes nothing in Portainer; another address
// replaces the environment.
func TestEnvironmentResource_EdgeTunnelAdopted(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeEnvironments(mock)
	cfg := func(extra string) string {
		return mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_environment" "e" {
  name                = "edge"
  environment_address = "https://portainer.example.com"
  type                = 4
  %s
}
`, extra)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: cfg("")},
			{
				Config: cfg(`edge_tunnel_server_address = "portainer.example.com:8000"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("portainer_environment.e", plancheck.ResourceActionUpdate),
				}},
				Check: checkEnv(func() error {
					if environmentCount(mock, "PUT", "/endpoints/1") != 0 {
						return fmt.Errorf("adopting the tunnel address must not update the environment")
					}
					return nil
				}),
			},
			{
				Config: cfg(`edge_tunnel_server_address = "other.example.com:8000"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("portainer_environment.e", plancheck.ResourceActionDestroyBeforeCreate),
				}},
			},
		},
	})
}

// TestEnvironmentResource_BusinessEditionOnly: async mode and the async
// intervals fail clearly on Community Edition, which would ignore them.
func TestEnvironmentResource_BusinessEditionOnly(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mockServerEdition(mock, "CE")
	newFakeEnvironments(mock)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
resource "portainer_environment" "e" {
  name                = "edge"
  environment_address = "https://portainer.example.com"
  type                = 4
  edge_async_mode     = true
  edge_ping_interval  = 30
}
`,
			ExpectError: regexp.MustCompile(`edge_async_mode, edge_ping_interval require Portainer Business Edition`),
		}},
	})
}

// TestEnvironmentResource_Azure: the Azure credentials (the key write-only)
// go into the create request; an update sends only the changed ones, and the
// write-only key only when its version changes.
func TestEnvironmentResource_Azure(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEnvironments(mock)
	cfg := func(tenant, key string, version int) string {
		return mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_environment" "e" {
  name                                = "aci"
  environment_address                 = "https://management.azure.com"
  type                                = 3
  azure_application_id                = "app"
  azure_tenant_id                     = %q
  azure_authentication_key_wo         = %q
  azure_authentication_key_wo_version = %d
}
`, tenant, key, version)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: cfg("tenant", "SECRET", 1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("portainer_environment.e", "azure_authentication_key_wo"),
					resource.TestCheckNoResourceAttr("portainer_environment.e", "azure_authentication_key"),
					resource.TestCheckResourceAttr("portainer_environment.e", "azure_tenant_id", "tenant"),
					checkEnv(func() error {
						got := fake.lastCreate()
						if got["EndpointCreationType"] != "3" || got["AzureApplicationID"] != "app" || got["AzureTenantID"] != "tenant" || got["AzureAuthenticationKey"] != "SECRET" {
							return fmt.Errorf("azure create fields: %v", got)
						}
						return nil
					}),
				),
			},
			{Config: cfg("tenant", "SECRET-2", 1), PlanOnly: true},
			{
				Config: cfg("tenant-2", "SECRET-2", 1),
				Check: checkEnv(func() error {
					p := environmentLastPut(t, mock, "1")
					if p["AzureTenantID"] != "tenant-2" {
						return fmt.Errorf("AzureTenantID: %v", p)
					}
					for _, k := range []string{"AzureApplicationID", "AzureAuthenticationKey"} {
						if _, ok := p[k]; ok {
							return fmt.Errorf("unchanged %s must not be sent: %v", k, p)
						}
					}
					return nil
				}),
			},
			{
				Config: cfg("tenant-2", "SECRET-2", 2),
				Check: checkEnv(func() error {
					p := environmentLastPut(t, mock, "1")
					if p["AzureAuthenticationKey"] != "SECRET-2" {
						return fmt.Errorf("AzureAuthenticationKey: %v", p)
					}
					fake.mu.Lock()
					defer fake.mu.Unlock()
					if got := fake.envs[1].azure; got != [3]string{"app", "tenant-2", "SECRET-2"} {
						return fmt.Errorf("stored credentials: %v", got)
					}
					return nil
				}),
			},
			{
				ResourceName:      "portainer_environment.e",
				ImportState:       true,
				ImportStateVerify: true,
				// Portainer returns no credentials and no TLS configuration for
				// Azure environments.
				ImportStateVerifyIgnore: []string{"azure_application_id", "azure_tenant_id", "azure_authentication_key_wo_version", "tls_enabled", "tls_skip_verify", "tls_skip_client_verify"},
			},
		},
	})
}

// TestEnvironmentResource_CustomTemplate: the custom template goes into the
// create request; changing it later sends nothing and replaces nothing.
func TestEnvironmentResource_CustomTemplate(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mockServerEdition(mock, "EE")
	fake := newFakeEnvironments(mock)
	cfg := func(id int) string {
		return mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_environment" "e" {
  name                       = "k8s"
  environment_address        = "https://k8s.example:6443"
  type                       = 5
  custom_template_id         = %d
  custom_template_stack_name = "bootstrap"
}
`, id)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: cfg(3),
				Check: checkEnv(func() error {
					got := fake.lastCreate()
					if got["CustomTemplateID"] != "3" || got["StackName"] != "bootstrap" {
						return fmt.Errorf("custom template create fields: %v", got)
					}
					if _, ok := got["CustomTemplateContent"]; ok {
						return fmt.Errorf("unset CustomTemplateContent must not be sent: %v", got)
					}
					return nil
				}),
			},
			{
				Config: cfg(4),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("portainer_environment.e", plancheck.ResourceActionUpdate),
				}},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_environment.e", "custom_template_id", "4"),
					checkEnv(func() error {
						if environmentCount(mock, "PUT", "/endpoints/1") != 0 || environmentCount(mock, "POST", "/endpoints") != 1 {
							return fmt.Errorf("a custom template change must neither update nor recreate the environment")
						}
						return nil
					}),
				),
			},
		},
	})
}

// envAgent is the configuration of issue #165: an Agent environment with the
// default TLS flags (on, both skip flags true).
func envAgent(name string, group int, address, policies string) string {
	return fmt.Sprintf(`
resource "portainer_environment" "e" {
  name                 = %q
  group_id             = %d
  environment_address  = %q
  type                 = 2
  user_access_policies = %s
}
`, name, group, address, policies)
}

// TestEnvironmentResource_AgentOnBusinessEdition (issue #165): on Business
// Edition, an update carrying tls with a true skip flag fails with 500 when
// the agent environment has no CA or client certificate file to delete. The
// create with access policies (Create's follow-up update), a rename, a group
// change, an address change and a policy removal all succeed, and none of
// those updates sends a skip flag.
func TestEnvironmentResource_AgentOnBusinessEdition(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEnvironments(mock)
	fake.businessEdition = true
	noSkipFlags := checkEnv(func() error {
		p := environmentLastPut(t, mock, "1")
		if p == nil {
			return fmt.Errorf("expected PUT /endpoints/1")
		}
		if p["tls"] != true {
			return fmt.Errorf("tls must still be sent: %v", p)
		}
		for _, k := range []string{"tlsskipVerify", "tlsskipClientVerify"} {
			if _, ok := p[k]; ok {
				return fmt.Errorf("unchanged %s must not be sent: %v", k, p)
			}
		}
		return nil
	})
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + envAgent("test", 1, "tcp://agent:9001", `{ "2" = 1 }`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_environment.e", "user_access_policies.2", "1"),
					resource.TestCheckResourceAttr("portainer_environment.e", "tls_skip_verify", "true"),
					resource.TestCheckResourceAttr("portainer_environment.e", "tls_skip_client_verify", "true"),
					noSkipFlags,
				),
			},
			{
				Config: mockProviderConfig(mock) + envAgent("test-renamed", 2, "tcp://agent:9001", `{ "2" = 1 }`),
				Check:  resource.ComposeTestCheckFunc(resource.TestCheckResourceAttr("portainer_environment.e", "name", "test-renamed"), noSkipFlags),
			},
			{
				Config: mockProviderConfig(mock) + envAgent("test-renamed", 2, "tcp://agent2:9001", `{}`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_environment.e", "environment_address", "tcp://agent2:9001"),
					resource.TestCheckResourceAttr("portainer_environment.e", "user_access_policies.%", "0"),
					noSkipFlags,
				),
			},
		},
	})
}

// TestEnvironmentResource_AdoptTLSFlags: Create adopting an existing
// environment by name sends a true skip flag only when that environment does
// not skip yet (its CA or client certificate is then deleted, which works on
// Business Edition), and leaves out the flags it already has.
func TestEnvironmentResource_AdoptTLSFlags(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		existing portainerTLSConfig
		want     map[string]bool // flags expected in the PUT; others must be absent
	}{
		{"already skipping", portainerTLSConfig{TLS: true, TLSSkipVerify: true}, map[string]bool{}},
		{"verifying with client cert", portainerTLSConfig{TLS: true, TLSCert: "/data/tls/7/cert.pem"}, map[string]bool{"tlsskipVerify": true, "tlsskipClientVerify": true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mock := NewMockServer(t)
			fake := newFakeEnvironments(mock)
			fake.businessEdition = true
			tls := tc.existing
			fake.add(&fakeEnvironment{ID: 7, Name: "test", Type: 2, GroupID: 1, URL: "agent:9001", TLSConfig: &tls})
			resource.UnitTest(t, resource.TestCase{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Steps: []resource.TestStep{{
					Config: mockProviderConfig(mock) + envAgent("test", 1, "tcp://agent:9001", `{}`),
					Check: resource.ComposeTestCheckFunc(
						resource.TestCheckResourceAttr("portainer_environment.e", "id", "7"),
						checkEnv(func() error {
							p := environmentLastPut(t, mock, "7")
							if p == nil {
								return fmt.Errorf("expected PUT /endpoints/7")
							}
							for _, k := range []string{"tlsskipVerify", "tlsskipClientVerify"} {
								v, sent := p[k]
								if tc.want[k] != sent || (sent && v != true) {
									return fmt.Errorf("%s: sent=%v value=%v, want sent=%v: %v", k, sent, v, tc.want[k], p)
								}
							}
							return nil
						}),
					),
				}},
			})
		})
	}
}
