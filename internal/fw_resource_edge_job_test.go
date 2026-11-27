package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// fakeEdgeJobs is a stateful stand-in for Portainer's edge job API. A job
// keeps its endpoints in a map, as Portainer does.
type fakeEdgeJobs struct {
	mu     sync.Mutex
	nextID int
	jobs   map[int]map[string]interface{}
	// formFields holds the multipart fields of the last file upload.
	formFields map[string]string
}

func newFakeEdgeJobs(m *MockServer, ids ...int) *fakeEdgeJobs {
	f := &fakeEdgeJobs{jobs: map[int]map[string]interface{}{}}
	m.On("GET", "/edge_jobs", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		list := make([]map[string]interface{}, 0, len(f.jobs))
		for id, j := range f.jobs {
			list = append(list, map[string]interface{}{"Id": id, "Name": j["Name"], "CronExpression": j["CronExpression"]})
		}
		RespondJSON(http.StatusOK, list)(w, r)
	})
	m.On("POST", "/edge_jobs/create/string", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.nextID++
		f.jobs[f.nextID] = edgeJobFromPayload(body)
		RespondJSON(http.StatusOK, map[string]interface{}{"Id": f.nextID})(w, r)
	})
	m.On("POST", "/edge_jobs/create/file", func(w http.ResponseWriter, r *http.Request) {
		_, params, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		mr := multipart.NewReader(r.Body, params["boundary"])
		fields := map[string]string{}
		for {
			part, err := mr.NextPart()
			if err != nil {
				break
			}
			data, _ := io.ReadAll(part)
			fields[part.FormName()] = string(data)
		}
		var groups, endpoints []interface{}
		_ = json.Unmarshal([]byte(fields["EdgeGroups"]), &groups)
		_ = json.Unmarshal([]byte(fields["Endpoints"]), &endpoints)
		recurring, _ := strconv.ParseBool(fields["Recurring"])
		f.mu.Lock()
		defer f.mu.Unlock()
		f.formFields = fields
		f.nextID++
		f.jobs[f.nextID] = edgeJobFromPayload(map[string]interface{}{
			"name": fields["Name"], "cronExpression": fields["CronExpression"],
			"edgeGroups": groups, "endpoints": endpoints, "recurring": recurring,
		})
		RespondJSON(http.StatusOK, map[string]interface{}{"Id": f.nextID})(w, r)
	})
	for _, id := range ids {
		id := id
		p := "/edge_jobs/" + strconv.Itoa(id)
		m.On("GET", p, func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			j, ok := f.jobs[id]
			if !ok {
				RespondString(http.StatusNotFound, "", "not found")(w, r)
				return
			}
			RespondJSON(http.StatusOK, j)(w, r)
		})
		m.On("PUT", p, func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			defer f.mu.Unlock()
			f.jobs[id] = edgeJobFromPayload(body)
			RespondJSON(http.StatusOK, map[string]interface{}{})(w, r)
		})
		m.On("DELETE", p, func(w http.ResponseWriter, _ *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			delete(f.jobs, id)
			w.WriteHeader(http.StatusNoContent)
		})
	}
	return f
}

func edgeJobFromPayload(body map[string]interface{}) map[string]interface{} {
	endpoints := map[string]interface{}{}
	if list, ok := body["endpoints"].([]interface{}); ok {
		for _, e := range list {
			endpoints[fmt.Sprint(e)] = map[string]interface{}{}
		}
	}
	return map[string]interface{}{
		"Name": body["name"], "CronExpression": body["cronExpression"], "EdgeGroups": body["edgeGroups"],
		"Endpoints": endpoints, "Recurring": body["recurring"],
	}
}

const edgeJobStringHCL = `resource "portainer_edge_job" "j" {
  name            = "cleanup"
  cron_expression = "0 * * * *"
  edge_groups     = [1, 2]
  endpoints       = [12, 10, 11]
  recurring       = true
  file_content    = "#!/bin/bash\necho hello"
}`

func TestEdgeJobResource_StringLifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeEdgeJobs(mock, 1)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + edgeJobStringHCL,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_edge_job.j", "id", "1"),
					resource.TestCheckResourceAttr("portainer_edge_job.j", "endpoints.0", "12"),
				),
			},
			{
				// The endpoints come back from a map; the configured order holds.
				Config:   mockProviderConfig(mock) + edgeJobStringHCL,
				PlanOnly: true,
			},
			{
				Config: mockProviderConfig(mock) + `resource "portainer_edge_job" "j" {
  name            = "cleanup"
  cron_expression = "5 * * * *"
  edge_groups     = [1]
  endpoints       = []
  file_content    = "echo bye"
}`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_edge_job.j", plancheck.ResourceActionUpdate)},
				},
			},
			{
				ResourceName:            "portainer_edge_job.j",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"file_content"},
			},
		},
	})

	var create, update map[string]interface{}
	if err := mock.FindRequest("POST", "/edge_jobs/create/string").DecodeJSON(&create); err != nil {
		t.Fatal(err)
	}
	if create["name"] != "cleanup" || create["cronExpression"] != "0 * * * *" || create["recurring"] != true || create["fileContent"] != "#!/bin/bash\necho hello" {
		t.Errorf("unexpected create payload %v", create)
	}
	if err := mock.FindRequest("PUT", "/edge_jobs/1").DecodeJSON(&update); err != nil {
		t.Fatal(err)
	}
	if update["cronExpression"] != "5 * * * *" || update["recurring"] != false || update["fileContent"] != "echo bye" {
		t.Errorf("unexpected update payload %v", update)
	}
}

func TestEdgeJobResource_FromFilePath(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEdgeJobs(mock, 1)
	fp := filepath.Join(t.TempDir(), "job.sh")
	if err := os.WriteFile(fp, []byte("#!/bin/sh\necho run\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + fmt.Sprintf(`resource "portainer_edge_job" "j" {
  name            = "fromfile"
  cron_expression = "0 * * * *"
  edge_groups     = [1]
  endpoints       = [10]
  file_path       = %q
}`, fp),
			Check: resource.TestCheckResourceAttr("portainer_edge_job.j", "id", "1"),
		}},
	})
	if fake.formFields["file"] != "#!/bin/sh\necho run\n" || fake.formFields["EdgeGroups"] != "[1]" || fake.formFields["Recurring"] != "false" {
		t.Errorf("unexpected multipart fields %v", fake.formFields)
	}
}

func TestEdgeJobResource_ExistingNameIsAdopted(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEdgeJobs(mock, 42)
	fake.jobs[42] = edgeJobFromPayload(map[string]interface{}{"name": "cleanup"})

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + edgeJobStringHCL,
			Check:  resource.TestCheckResourceAttr("portainer_edge_job.j", "id", "42"),
		}},
	})
	if mock.FindRequest("POST", "/edge_jobs/create/string") != nil {
		t.Error("an existing job must not be created again")
	}
	if mock.FindRequest("PUT", "/edge_jobs/42") == nil {
		t.Error("expected the existing job to be updated")
	}
}

func TestEdgeJobResource_Errors(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		setup  func(*MockServer)
		config string
		want   string
	}{
		"no source": {
			config: `resource "portainer_edge_job" "j" {
  name            = "x"
  cron_expression = "* * * * *"
  edge_groups     = [1]
  endpoints       = []
}`,
			want: `(Exactly one of these attributes|Invalid Attribute Combination)`,
		},
		"both sources": {
			config: `resource "portainer_edge_job" "j" {
  name            = "x"
  cron_expression = "* * * * *"
  edge_groups     = [1]
  endpoints       = []
  file_content    = "a"
  file_path       = "/b"
}`,
			want: `(Exactly one of these attributes|Invalid Attribute Combination)`,
		},
		"list error": {
			setup: func(m *MockServer) {
				m.On("GET", "/edge_jobs", RespondString(http.StatusInternalServerError, "", "boom"))
			},
			config: edgeJobStringHCL,
			want:   `failed to check for existing edge job`,
		},
		"create error": {
			setup: func(m *MockServer) {
				m.On("GET", "/edge_jobs", RespondJSON(http.StatusOK, []interface{}{}))
				m.On("POST", "/edge_jobs/create/string", RespondString(http.StatusBadRequest, "", "invalid cron"))
			},
			config: edgeJobStringHCL,
			want:   `(?s)failed to create edge job:.*invalid cron`,
		},
		"file open error": {
			setup: func(m *MockServer) {
				m.On("GET", "/edge_jobs", RespondJSON(http.StatusOK, []interface{}{}))
			},
			config: `resource "portainer_edge_job" "j" {
  name            = "x"
  cron_expression = "* * * * *"
  edge_groups     = [1]
  endpoints       = []
  file_path       = "/nonexistent/job.sh"
}`,
			want: `cannot open file`,
		},
		"file upload error": {
			setup: func(m *MockServer) {
				m.On("GET", "/edge_jobs", RespondJSON(http.StatusOK, []interface{}{}))
				m.On("POST", "/edge_jobs/create/file", RespondString(http.StatusBadRequest, "", "bad upload"))
			},
			config: `resource "portainer_edge_job" "j" {
  name            = "x"
  cron_expression = "* * * * *"
  edge_groups     = [1]
  endpoints       = []
  file_path       = "fw_resource_edge_job_test.go"
}`,
			want: `failed to create edge job from file: bad upload`,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			if tc.setup != nil {
				tc.setup(mock)
			}
			resource.UnitTest(t, resource.TestCase{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Steps: []resource.TestStep{{
					Config:      mockProviderConfig(mock) + tc.config,
					ExpectError: regexp.MustCompile(tc.want),
				}},
			})
		})
	}
}

func TestEdgeJobResource_DeleteNeeds204(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEdgeJobs(mock)
	fake.jobs[1] = edgeJobFromPayload(map[string]interface{}{"name": "cleanup"})
	gone := false
	mock.On("GET", "/edge_jobs/1", func(w http.ResponseWriter, r *http.Request) {
		RespondJSON(http.StatusOK, edgeJobFromPayload(map[string]interface{}{
			"name": "cleanup", "cronExpression": "0 * * * *", "edgeGroups": []int{1, 2},
			"endpoints": []interface{}{12, 10, 11}, "recurring": true,
		}))(w, r)
	})
	mock.On("PUT", "/edge_jobs/1", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("DELETE", "/edge_jobs/1", func(w http.ResponseWriter, r *http.Request) {
		if !gone {
			RespondJSON(http.StatusOK, map[string]interface{}{})(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: mockProviderConfig(mock) + edgeJobStringHCL},
			{Config: mockProviderConfig(mock), ExpectError: regexp.MustCompile(`failed to delete edge job`)},
			{PreConfig: func() { gone = true }, Config: mockProviderConfig(mock)},
		},
	})
}

// TestEdgeJob_UpgradeFromV300: state written by v3.0.0 (SDKv2) plans no
// change with this build, with recurring set and left out.
func TestEdgeJob_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeEdgeJobs(mock, 1, 2)

	// One endpoint each: v3.0.0 itself read several back in random order.
	config := mockProviderConfig(mock) + `
resource "portainer_edge_job" "j" {
  name            = "cleanup"
  cron_expression = "0 * * * *"
  edge_groups     = [1, 2]
  endpoints       = [12]
  recurring       = true
  file_content    = "#!/bin/bash\necho hello"
}
resource "portainer_edge_job" "bare" {
  name            = "bare"
  cron_expression = "0 0 * * *"
  edge_groups     = [3]
  endpoints       = []
  file_content    = "true"
}`
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

// TestAccEdgeJobResource_basic needs edge environments to target, which a
// plain CE instance does not have.
func TestAccEdgeJobResource_basic(t *testing.T) {
	if os.Getenv("PORTAINER_ACC_EDGE_ENV_ID") == "" {
		t.Skip("PORTAINER_ACC_EDGE_ENV_ID must name an edge environment for the edge job acceptance test")
	}
	accPreCheck(t)
	config := accProvider() + fmt.Sprintf(`resource "portainer_edge_job" "j" {
  name            = "acc-edge-job"
  cron_expression = "0 0 * * *"
  edge_groups     = []
  endpoints       = [%s]
  file_content    = "true"
}`, os.Getenv("PORTAINER_ACC_EDGE_ENV_ID"))
	resource.Test(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{ResourceName: "portainer_edge_job.j", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"file_content"}},
		},
	})
}
