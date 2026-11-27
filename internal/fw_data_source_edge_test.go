package internal

import (
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// Framework data sources of the edge, D2K and KubeSolo batch, run through
// real Terraform against canned responses.

func edgeDSRun(t *testing.T, mock *MockServer, config string, checks ...resource.TestCheckFunc) {
	t.Helper()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + config,
			Check:  resource.ComposeTestCheckFunc(checks...),
		}},
	})
}

func edgeDSError(t *testing.T, mock *MockServer, config, want string) {
	t.Helper()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + config,
			ExpectError: edgeErrRe(want),
		}},
	})
}

// TestEdgeLookupDataSources covers the four look-ups by name: found, not
// found, and a failed listing.
func TestEdgeLookupDataSources(t *testing.T) {
	t.Parallel()
	cases := []struct {
		tfType, list string
		items        interface{}
		checks       map[string]string
		what         string
	}{
		{
			tfType: "portainer_edge_group", list: "/edge_groups", what: "edge group",
			items:  []map[string]interface{}{{"Id": 1, "Name": "other"}, {"Id": 3, "Name": "prod", "Dynamic": true}},
			checks: map[string]string{"id": "3", "dynamic": "true"},
		},
		{
			tfType: "portainer_edge_job", list: "/edge_jobs", what: "edge job",
			items:  []map[string]interface{}{{"Id": 4, "Name": "prod", "CronExpression": "0 * * * *"}},
			checks: map[string]string{"id": "4", "cron_expression": "0 * * * *"},
		},
		{
			tfType: "portainer_edge_stack", list: "/edge_stacks", what: "edge stack",
			items:  []map[string]interface{}{{"Id": 5, "Name": "prod", "DeploymentType": 1}},
			checks: map[string]string{"id": "5", "deployment_type": "1"},
		},
		{
			tfType: "portainer_edge_configuration", list: "/edge_configurations", what: "edge configuration",
			items:  []map[string]interface{}{{"id": 6, "name": "prod", "type": 1, "category": "secret"}},
			checks: map[string]string{"id": "6", "type": "1", "category": "secret"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.tfType, func(t *testing.T) {
			mock := NewMockServer(t)
			mock.On("GET", tc.list, RespondJSON(http.StatusOK, tc.items))
			var checks []resource.TestCheckFunc
			for k, v := range tc.checks {
				checks = append(checks, resource.TestCheckResourceAttr("data."+tc.tfType+".x", k, v))
			}
			edgeDSRun(t, mock, `data "`+tc.tfType+`" "x" { name = "prod" }`, checks...)
			edgeDSError(t, mock, `data "`+tc.tfType+`" "x" { name = "missing" }`, tc.what+" missing not found")

			broken := NewMockServer(t)
			broken.On("GET", tc.list, RespondString(http.StatusInternalServerError, "", "boom"))
			edgeDSError(t, broken, `data "`+tc.tfType+`" "x" { name = "prod" }`, "failed to list "+tc.what+"s")
		})
	}
}

func TestKubeSoloDataSources(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/kubesolo/versions", RespondJSON(http.StatusOK, map[string]interface{}{
		"bundled": "v1.2.3", "minimum": "v1.2.0", "versions": []string{"v1.2.4", "v1.2.3"}, "githubError": "GitHub answered HTTP 502",
	}))
	mock.On("GET", "/kubesolo/offline-bundles", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"name": "kubesolo-v1.2.3-linux-amd64-offline.tar.gz", "arch": "amd64", "libc": "glibc", "size": 3},
	}))
	mock.On("GET", "/kubesolo/environments/7/config", RespondJSON(http.StatusOK, map[string]interface{}{
		"api": "available", "version": "v1.2.2", "readAt": 1767225600, "config": map[string]interface{}{"portainerEdgeKey": "secret-key"},
	}))
	mock.On("GET", "/kubesolo/environments/8/config", RespondJSON(http.StatusOK, map[string]interface{}{
		"api": "unreachable", "readAt": 1, "error": "connection refused",
	}))
	edgeDSRun(t, mock, `
data "portainer_kubesolo_versions" "v" {}
data "portainer_kubesolo_offline_bundles" "b" {}
data "portainer_kubesolo_config" "c" { environment_id = 7 }
data "portainer_kubesolo_config" "none" { environment_id = 8 }
`,
		resource.TestCheckResourceAttr("data.portainer_kubesolo_versions.v", "id", "portainer-kubesolo-versions"),
		resource.TestCheckResourceAttr("data.portainer_kubesolo_versions.v", "bundled", "v1.2.3"),
		resource.TestCheckResourceAttr("data.portainer_kubesolo_versions.v", "versions.0", "v1.2.4"),
		resource.TestCheckResourceAttr("data.portainer_kubesolo_versions.v", "github_error", "GitHub answered HTTP 502"),
		resource.TestCheckResourceAttr("data.portainer_kubesolo_offline_bundles.b", "bundles.0.libc", "glibc"),
		resource.TestCheckResourceAttr("data.portainer_kubesolo_offline_bundles.b", "bundles.0.size", "3"),
		resource.TestCheckResourceAttr("data.portainer_kubesolo_config.c", "id", "7"),
		resource.TestCheckResourceAttr("data.portainer_kubesolo_config.c", "read_at", "1767225600"),
		resource.TestCheckResourceAttr("data.portainer_kubesolo_config.c", "config_json", `{"portainerEdgeKey":"secret-key"}`),
		resource.TestCheckResourceAttr("data.portainer_kubesolo_config.none", "config_json", ""),
		resource.TestCheckResourceAttr("data.portainer_kubesolo_config.none", "error", "connection refused"),
	)

	// A pre-3.0 server's 404 is explained; on a 3.0 server it stays a plain
	// not-found.
	old := NewMockServer(t)
	old.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{"ServerVersion": "2.45.0", "ServerEdition": "BE"}))
	edgeDSError(t, old, `data "portainer_kubesolo_versions" "v" {}`, "portainer_kubesolo_versions requires Portainer Business Edition 3.0 or later")
	edgeDSError(t, old, `data "portainer_kubesolo_offline_bundles" "b" {}`, "failed to list KubeSolo offline bundles")
	current := NewMockServer(t)
	current.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{"ServerVersion": "3.0.0", "ServerEdition": "BE"}))
	edgeDSError(t, current, `data "portainer_kubesolo_config" "c" { environment_id = 8 }`, "failed to read the KubeSolo configuration of environment 8")
}

func TestEdgeMTLSCertificateDataSources(t *testing.T) {
	t.Parallel()
	cert := map[string]interface{}{
		"Subject":                map[string]interface{}{"CommonName": "portainer-edge-ca", "Organization": []string{"Example"}},
		"Issuer":                 map[string]interface{}{"CommonName": "portainer-edge-ca"},
		"SerialNumber":           "42",
		"SHA256Fingerprint":      "ab:cd",
		"SignatureAlgorithm":     "SHA256-RSA",
		"IsCertificateAuthority": true,
		"ValidNotBefore":         "2026-01-01T00:00:00Z",
		"ValidNotAfter":          "2027-01-01T00:00:00Z",
		"Version":                3,
		"SubjectAltDNSNames":     []string{"edge.example.com"},
		"SubjectAltIpAddresses":  []string{"10.0.0.1"},
		"KeyUsages":              []string{"CertSign"},
		"ExtendedKeyUsages":      []string{"ServerAuth"},
		"PublicKey":              map[string]interface{}{"Algorithm": "RSA", "Size": 4096},
	}
	mock := NewMockServer(t)
	mock.On("GET", "/settings/edge/mtls_ca_certificate", RespondJSON(http.StatusOK, map[string]interface{}{"MTLSCACertificate": cert}))
	mock.On("GET", "/settings/edge/mtls_certificate", RespondJSON(http.StatusOK, map[string]interface{}{"MTLSCertificate": cert}))
	edgeDSRun(t, mock, `
data "portainer_edge_mtls_ca_certificate" "ca" {}
data "portainer_edge_mtls_certificate" "leaf" {}
`,
		resource.TestCheckResourceAttr("data.portainer_edge_mtls_ca_certificate.ca", "configured", "true"),
		resource.TestCheckResourceAttr("data.portainer_edge_mtls_ca_certificate.ca", "id", "portainer-edge-mtls-ca-certificate"),
		resource.TestCheckResourceAttr("data.portainer_edge_mtls_ca_certificate.ca", "organization.0", "Example"),
		resource.TestCheckResourceAttr("data.portainer_edge_mtls_ca_certificate.ca", "public_key_size", "4096"),
		resource.TestCheckResourceAttr("data.portainer_edge_mtls_certificate.leaf", "subject_alt_ip_addresses.0", "10.0.0.1"),
		resource.TestCheckResourceAttr("data.portainer_edge_mtls_certificate.leaf", "valid_not_after", "2027-01-01T00:00:00Z"),
	)

	// No certificate configured: an empty object or a 404 both read as such.
	none := NewMockServer(t)
	none.On("GET", "/settings/edge/mtls_ca_certificate", RespondJSON(http.StatusOK, map[string]interface{}{}))
	edgeDSRun(t, none, `
data "portainer_edge_mtls_ca_certificate" "ca" {}
data "portainer_edge_mtls_certificate" "leaf" {}
`,
		resource.TestCheckResourceAttr("data.portainer_edge_mtls_ca_certificate.ca", "configured", "false"),
		resource.TestCheckResourceAttr("data.portainer_edge_mtls_ca_certificate.ca", "common_name", ""),
		resource.TestCheckResourceAttr("data.portainer_edge_mtls_certificate.leaf", "configured", "false"),
		resource.TestCheckResourceAttr("data.portainer_edge_mtls_certificate.leaf", "key_usages.#", "0"),
	)

	// Only a missing certificate is shrugged off, not a broken call.
	forbidden := NewMockServer(t)
	forbidden.On("GET", "/settings/edge/mtls_certificate", RespondString(http.StatusForbidden, "application/json", `{"message":"forbidden"}`))
	edgeDSError(t, forbidden, `data "portainer_edge_mtls_certificate" "leaf" {}`, "failed to read the edge mTLS certificate")
}

func TestEdgeWaitingRoomDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 12, "Name": "shop-floor-3", "Type": 4, "EdgeID": "abc", "GroupId": 1, "LastCheckInDate": 1756000000},
		{"Id": 13, "Name": "shop-floor-4", "Type": 7, "EdgeID": "def", "GroupId": 1, "LastCheckInDate": 1756000100},
	}))
	edgeDSRun(t, mock, `data "portainer_edge_waiting_room" "w" {}`,
		resource.TestCheckResourceAttr("data.portainer_edge_waiting_room.w", "endpoint_ids.0", "12"),
		resource.TestCheckResourceAttr("data.portainer_edge_waiting_room.w", "endpoint_ids.1", "13"),
		resource.TestCheckResourceAttr("data.portainer_edge_waiting_room.w", "environments.0.edge_id", "abc"),
		resource.TestCheckResourceAttr("data.portainer_edge_waiting_room.w", "environments.0.last_check_in", "1756000000"),
	)
	if req := mock.FindRequest("GET", "/endpoints"); req == nil || !strings.Contains(req.Query, "edgeDeviceUntrusted=true") || strings.Contains(req.Query, "groupIds") {
		t.Errorf("expected only the untrusted filter, got %+v", req)
	}

	// The optional group narrowing; an empty waiting room is an empty list.
	empty := NewMockServer(t)
	empty.On("GET", "/endpoints", RespondJSON(http.StatusOK, []interface{}{}))
	edgeDSRun(t, empty, `data "portainer_edge_waiting_room" "w" { group_id = 3 }`,
		resource.TestCheckResourceAttr("data.portainer_edge_waiting_room.w", "endpoint_ids.#", "0"),
	)
	if req := empty.FindRequest("GET", "/endpoints"); req == nil || !strings.Contains(req.Query, "groupIds=3") {
		t.Errorf("expected the group filter in the query, got %+v", req)
	}
}

func TestEdgeUpdateDataSources(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/edge_update_schedules/info", RespondJSON(http.StatusOK, map[string]interface{}{
		"MinAgentVersion": "2.19.0", "UpToDateCount": 12, "OutdatedCount": 3, "HasLocalTimeZone": true,
	}))
	mock.On("GET", "/edge_update_schedules/agent_versions", RespondJSON(http.StatusOK, []string{"2.21.0", "2.20.0", "2.19.0"}))
	mock.On("GET", "/edge_update_schedules/previous_versions", RespondJSON(http.StatusOK, map[string]string{"12": "2.20.0", "3": "2.19.0"}))
	mock.On("POST", "/edge_update_schedules/active", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"scheduleId": 2, "environmentId": 12, "edgeStackId": 8, "targetVersion": "2.21.0"},
	}))
	edgeDSRun(t, mock, `
data "portainer_edge_update_schedule_info" "i" {}
data "portainer_edge_update_previous_versions" "p" { environment_ids = [3, 12] }
data "portainer_edge_update_schedules_active" "a" { environment_ids = [12, 13] }
`,
		resource.TestCheckResourceAttr("data.portainer_edge_update_schedule_info.i", "outdated_count", "3"),
		// Sorted, so two identical plans produce identical output.
		resource.TestCheckResourceAttr("data.portainer_edge_update_schedule_info.i", "agent_versions.0", "2.19.0"),
		resource.TestCheckResourceAttr("data.portainer_edge_update_schedule_info.i", "has_no_local_time_zone", "false"),
		// Sorted by identifier as a string.
		resource.TestCheckResourceAttr("data.portainer_edge_update_previous_versions.p", "previous_versions.0.endpoint_id", "12"),
		resource.TestCheckResourceAttr("data.portainer_edge_update_previous_versions.p", "previous_versions.#", "2"),
		resource.TestCheckResourceAttr("data.portainer_edge_update_schedules_active.a", "schedules.0.target_version", "2.21.0"),
		resource.TestCheckResourceAttr("data.portainer_edge_update_schedules_active.a", "schedules.0.edge_stack_id", "8"),
	)
	if query := mock.FindRequest("GET", "/edge_update_schedules/previous_versions").Query; !strings.Contains(query, "environmentIds=3%2C12") || strings.Contains(query, "edgeGroupIds") {
		t.Errorf("expected the identifiers joined with a comma, got %q", query)
	}
	var payload struct{ EnvironmentIDs []int }
	if err := mock.FindRequest("POST", "/edge_update_schedules/active").DecodeJSON(&payload); err != nil || len(payload.EnvironmentIDs) != 2 {
		t.Errorf("the environments were not sent: %v %v", payload.EnvironmentIDs, err)
	}
}

func TestEdgeFileAndStatusDataSources(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/edge_stacks/4/file", RespondJSON(http.StatusOK, map[string]interface{}{"StackFileContent": "edge stack"}))
	mock.On("GET", "/edge_jobs/5/file", RespondJSON(http.StatusOK, map[string]interface{}{"FileContent": "#!/bin/sh"}))
	mock.On("GET", "/edge_configurations/4/files", RespondString(http.StatusOK, "application/octet-stream", "BASE_DOMAIN=apps.example.com\n"))
	mock.On("GET", "/edge_stacks/9/stagger/status", RespondJSON(http.StatusOK, map[string]interface{}{"status": "in-progress"}))
	edgeDSRun(t, mock, `
data "portainer_edge_stack_file" "s" { edge_stack_id = 4 }
data "portainer_edge_job_file" "j" { edge_job_id = 5 }
data "portainer_edge_configuration_files" "c" { edge_configuration_id = 4 }
data "portainer_edge_stack_stagger_status" "st" { edge_stack_id = 9 }
`,
		// The edge stack endpoint uses StackFileContent, not FileContent.
		resource.TestCheckResourceAttr("data.portainer_edge_stack_file.s", "file_content", "edge stack"),
		resource.TestCheckResourceAttr("data.portainer_edge_stack_file.s", "id", "4/file"),
		resource.TestCheckResourceAttr("data.portainer_edge_job_file.j", "file_content", "#!/bin/sh"),
		resource.TestCheckResourceAttr("data.portainer_edge_configuration_files.c", "files", "BASE_DOMAIN=apps.example.com\n"),
		resource.TestCheckResourceAttr("data.portainer_edge_configuration_files.c", "id", "portainer-edge-configuration-files-4"),
		resource.TestCheckResourceAttr("data.portainer_edge_stack_stagger_status.st", "status", "in-progress"),
	)

	broken := NewMockServer(t)
	broken.On("GET", "/edge_configurations/4/files", RespondString(http.StatusForbidden, "", "forbidden"))
	edgeDSError(t, broken, `data "portainer_edge_configuration_files" "c" { edge_configuration_id = 4 }`, "failed to read the files of edge configuration 4")
	edgeDSError(t, broken, `data "portainer_edge_job_file" "j" { edge_job_id = 6 }`, "failed to read the file of edge job 6")
}

func TestEdgeJobTaskDataSources(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/edge_jobs/1/tasks", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": "1_2", "EndpointId": 2, "EndpointName": "branch-a", "LogsStatus": 3},
		{"Id": "1_3", "EndpointId": 3, "EndpointName": "branch-b", "LogsStatus": 1},
	}))
	mock.On("GET", "/edge_jobs/1/tasks/1_2/logs", RespondJSON(http.StatusOK, map[string]interface{}{"FileContent": "hello"}))
	// Logs not collected yet are absent, which is not an error.
	mock.On("GET", "/edge_jobs/1/tasks/1_3/logs", RespondString(http.StatusNotFound, "application/json", `{"message":"not found"}`))
	edgeDSRun(t, mock, `
data "portainer_edge_job_tasks" "t" { edge_job_id = 1 }
data "portainer_edge_job_task_logs" "collected" {
  edge_job_id = 1
  task_id     = "1_2"
}
data "portainer_edge_job_task_logs" "pending" {
  edge_job_id = 1
  task_id     = "1_3"
}
`,
		resource.TestCheckResourceAttr("data.portainer_edge_job_tasks.t", "id", "1/tasks"),
		resource.TestCheckResourceAttr("data.portainer_edge_job_tasks.t", "tasks.0.logs_collected", "true"),
		resource.TestCheckResourceAttr("data.portainer_edge_job_tasks.t", "tasks.1.logs_collected", "false"),
		resource.TestCheckResourceAttr("data.portainer_edge_job_tasks.t", "tasks.1.endpoint_name", "branch-b"),
		resource.TestCheckResourceAttr("data.portainer_edge_job_task_logs.collected", "logs", "hello"),
		resource.TestCheckResourceAttr("data.portainer_edge_job_task_logs.pending", "logs", ""),
		resource.TestCheckResourceAttr("data.portainer_edge_job_task_logs.pending", "id", "1/1_3/logs"),
	)

	broken := NewMockServer(t)
	broken.On("GET", "/edge_jobs/1/tasks/1_2/logs", RespondString(http.StatusInternalServerError, "", "boom"))
	edgeDSError(t, broken, `data "portainer_edge_job_task_logs" "l" {
  edge_job_id = 1
  task_id     = "1_2"
}`, "failed to read the logs of task 1_2 of edge job 1")
}

// TestAccEdgeDataSources reads the data sources a plain CE instance answers.
func TestAccEdgeDataSources(t *testing.T) {
	accPreCheck(t)
	resource.Test(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: accProvider() + `
data "portainer_edge_waiting_room" "w" {}
`,
			Check: resource.TestCheckResourceAttr("data.portainer_edge_waiting_room.w", "id", "portainer-edge-waiting-room"),
		}},
	})
}
