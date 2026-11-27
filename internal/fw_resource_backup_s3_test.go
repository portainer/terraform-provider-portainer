package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// fakeBackupS3 stores what POST /backup/s3/execute sent and returns it from
// GET /backup/s3/settings, with the secrets replaced by redact (Portainer
// 3.0 masks them) unless redact is empty and showSecrets is set.
type fakeBackupS3 struct {
	mu          sync.Mutex
	settings    map[string]interface{}
	redact      string
	showSecrets bool
}

func (f *fakeBackupS3) register(m *MockServer) {
	m.On("POST", "/backup/s3/execute", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.settings = body
		f.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	m.On("GET", "/backup/s3/settings", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		out := map[string]interface{}{}
		for k, v := range f.settings {
			out[k] = v
		}
		if !f.showSecrets {
			out["secretAccessKey"] = f.redact
			out["password"] = f.redact
		}
		RespondJSON(http.StatusOK, out)(w, r)
	})
	m.On("GET", "/backup/s3/status", RespondJSON(http.StatusOK, map[string]interface{}{"Failed": false, "TimestampUTC": "2026-01-01T00:00:00Z"}))
}

const backupS3Config = `
resource "portainer_backup_s3" "s" {
  access_key_id      = "AKIAEXAMPLE"
  secret_access_key  = "supersecret"
  bucket_name        = "portainer-backups"
  region             = "eu-central-1"
  s3_compatible_host = "https://s3.amazonaws.com"
  password           = "topsecret"
  %s
}`

// TestBackupS3Resource_Lifecycle: the payload carries every field, an unset
// cron_rule is left out, redacted secrets keep the configured ones and the
// next plan is empty.
func TestBackupS3Resource_Lifecycle(t *testing.T) {
	t.Parallel()
	for _, redact := range []string{"", "******"} {
		t.Run("redact="+redact, func(t *testing.T) {
			mock := NewMockServer(t)
			fake := &fakeBackupS3{redact: redact}
			fake.register(mock)

			resource.UnitTest(t, resource.TestCase{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Steps: []resource.TestStep{
					{
						Config: mockProviderConfig(mock) + fmt.Sprintf(backupS3Config, ""),
						Check: resource.ComposeTestCheckFunc(
							resource.TestCheckResourceAttr("portainer_backup_s3.s", "id", "portainer_backup_s3"),
							resource.TestCheckResourceAttr("portainer_backup_s3.s", "secret_access_key", "supersecret"),
							resource.TestCheckResourceAttr("portainer_backup_s3.s", "password", "topsecret"),
							resource.TestCheckNoResourceAttr("portainer_backup_s3.s", "cron_rule"),
							func(*terraform.State) error {
								body := gsbBody(t, mock, "POST", "/backup/s3/execute")
								want := map[string]interface{}{
									"accessKeyID": "AKIAEXAMPLE", "secretAccessKey": "supersecret", "bucketName": "portainer-backups",
									"region": "eu-central-1", "s3CompatibleHost": "https://s3.amazonaws.com", "password": "topsecret",
								}
								for k, v := range want {
									if body[k] != v {
										return fmt.Errorf("payload.%s: got %v, want %v", k, body[k], v)
									}
								}
								if _, ok := body["cronRule"]; ok {
									return fmt.Errorf("expected cronRule to be omitted when not set, got %v", body["cronRule"])
								}
								return nil
							},
						),
					},
					{
						Config: mockProviderConfig(mock) + fmt.Sprintf(backupS3Config, `cron_rule = "*/15 * * * *"`),
						Check: resource.ComposeTestCheckFunc(
							resource.TestCheckResourceAttr("portainer_backup_s3.s", "cron_rule", "*/15 * * * *"),
							func(*terraform.State) error {
								if c := gsbBody(t, mock, "POST", "/backup/s3/execute")["cronRule"]; c != "*/15 * * * *" {
									return fmt.Errorf("cronRule: got %v", c)
								}
								return nil
							},
						),
					},
					{
						RefreshState: true,
						Check: resource.ComposeTestCheckFunc(
							resource.TestCheckResourceAttr("portainer_backup_s3.s", "last_run_failed", "false"),
							resource.TestCheckResourceAttr("portainer_backup_s3.s", "last_run_timestamp", "2026-01-01T00:00:00Z"),
						),
					},
				},
			})
		})
	}
}

// TestBackupS3Resource_ReadPopulatesState: values Portainer reports differ
// from the configuration show as drift.
func TestBackupS3Resource_ReadPopulatesState(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := &fakeBackupS3{}
	fake.register(mock)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: mockProviderConfig(mock) + fmt.Sprintf(backupS3Config, "")},
			{
				PreConfig: func() {
					fake.mu.Lock()
					fake.settings["bucketName"] = "read-bucket"
					fake.settings["cronRule"] = "*/15 * * * *"
					fake.mu.Unlock()
				},
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_backup_s3.s", "bucket_name", "read-bucket"),
					resource.TestCheckResourceAttr("portainer_backup_s3.s", "cron_rule", "*/15 * * * *"),
				),
			},
		},
	})
}

func TestBackupS3Resource_CreateHTTPError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/backup/s3/execute", RespondString(http.StatusInternalServerError, "application/json", `{"message":"s3 upload failed"}`))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + fmt.Sprintf(backupS3Config, ""),
			ExpectError: regexp.MustCompile(`failed to execute S3 backup`),
		}},
	})
}

// TestBackupS3Resource_ReadErrors: an HTTP error or a body that is not JSON
// on GET /backup/s3/settings surfaces as an error.
func TestBackupS3Resource_ReadErrors(t *testing.T) {
	t.Parallel()
	for name, settings := range map[string]http.HandlerFunc{
		"http":   RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`),
		"decode": RespondString(http.StatusOK, "application/json", `not-json`),
	} {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			mock.On("POST", "/backup/s3/execute", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
			mock.On("GET", "/backup/s3/settings", settings)
			resource.UnitTest(t, resource.TestCase{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Steps: []resource.TestStep{{
					Config:      mockProviderConfig(mock) + fmt.Sprintf(backupS3Config, ""),
					ExpectError: regexp.MustCompile(`failed to read S3 backup settings`),
				}},
			})
		})
	}
}

// TestBackupS3Resource_WriteOnlySecrets: the write-only values reach
// Portainer, and Read does not copy the secrets Portainer returns into state.
func TestBackupS3Resource_WriteOnlySecrets(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := &fakeBackupS3{showSecrets: true}
	fake.register(mock)
	config := mockProviderConfig(mock) + `
resource "portainer_backup_s3" "s" {
  access_key_id                = "AKIA"
  secret_access_key_wo         = "s3-secret"
  secret_access_key_wo_version = 1
  bucket_name                  = "b"
  region                       = "eu-1"
  s3_compatible_host           = "s3.example.com"
  password_wo                  = "backup-pass"
  password_wo_version          = 1
}`

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					func(*terraform.State) error {
						body := gsbBody(t, mock, "POST", "/backup/s3/execute")
						if body["secretAccessKey"] != "s3-secret" || body["password"] != "backup-pass" {
							return fmt.Errorf("expected the write-only values, got %v", body)
						}
						return nil
					},
				),
			},
			{
				RefreshState: true,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("portainer_backup_s3.s", "secret_access_key"),
					resource.TestCheckNoResourceAttr("portainer_backup_s3.s", "password"),
					resource.TestCheckResourceAttr("portainer_backup_s3.s", "access_key_id", "AKIA"),
				),
			},
			{Config: config, PlanOnly: true},
		},
	})
}

func TestBackupS3Resource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	newMock := func(t *testing.T) *MockServer {
		mock := NewMockServer(t)
		(&fakeBackupS3{redact: "******"}).register(mock)
		return mock
	}
	gsbUpgradeTest(t, newMock,
		fmt.Sprintf(backupS3Config, ""),
		fmt.Sprintf(backupS3Config, `cron_rule = "0 2 * * *"`),
		`
resource "portainer_backup_s3" "s" {
  access_key_id                = "AKIA"
  secret_access_key_wo         = "s3-secret"
  secret_access_key_wo_version = 1
  bucket_name                  = "b"
  region                       = "eu-1"
  s3_compatible_host           = "s3.example.com"
  password_wo                  = "backup-pass"
  password_wo_version          = 1
}`,
	)
}
