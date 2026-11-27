package internal

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// Business Edition backup coverage. Portainer's backup API is split across
// three destinations with deliberately inconsistent JSON conventions: the
// settings payloads are camelCase, the restore payloads are PascalCase, and
// the status objects differ per destination (Azure and S3 capitalise their
// fields, local does not). Getting any of those wrong is silent - the request
// is accepted and the fields simply arrive empty - so the tests below pin the
// wire format rather than just the happy path.

// fakeEchoSettings stores what a settings POST sent and returns it from the
// GET of the same path, without the keys in hidden.
type fakeEchoSettings struct {
	mu       sync.Mutex
	settings map[string]interface{}
	hidden   []string
}

func (f *fakeEchoSettings) register(m *MockServer, path string) {
	m.On("POST", path, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.settings = body
		f.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	m.On("GET", path, func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		out := map[string]interface{}{}
		for k, v := range f.settings {
			out[k] = v
		}
		for _, k := range f.hidden {
			delete(out, k)
		}
		RespondJSON(http.StatusOK, out)(w, r)
	})
}

func (f *fakeEchoSettings) set(k string, v interface{}) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.settings[k] = v
}

func azureSettingsMock(t *testing.T) (*MockServer, *fakeEchoSettings) {
	mock := NewMockServer(t)
	fake := &fakeEchoSettings{hidden: []string{"storageAccountKey", "clientSecret", "clientCertificate", "clientCertificatePassword", "password"}}
	fake.register(mock, "/backup/azure/settings")
	mock.On("GET", "/backup/azure/status", RespondJSON(http.StatusOK, map[string]interface{}{
		"Failed": false, "TimestampUTC": "2026-09-15T02:00:00Z",
	}))
	return mock, fake
}

const azureSettingsConfig = `
resource "portainer_backup_azure_settings" "a" {
  auth_method          = "servicePrincipalSecret"
  storage_account_name = "acmebackups"
  container_name       = "portainer"
  tenant_id            = "tenant-1"
  client_id            = "client-1"
  client_secret        = "secret-1"
  cron_rule            = "0 2 * * *"
  password             = "archive-pass"
}`

// TestBackupAzureSettingsResource_Lifecycle pins the settings wire format,
// that unset optional fields are omitted rather than sent empty (which would
// clear a credential configured elsewhere), that the read does not overwrite
// configured credentials with what the server hands back, and the import.
func TestBackupAzureSettingsResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock, fake := azureSettingsMock(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + azureSettingsConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_backup_azure_settings.a", "id", "portainer-backup-azure-settings"),
					resource.TestCheckResourceAttr("portainer_backup_azure_settings.a", "last_run_timestamp", "2026-09-15T02:00:00Z"),
					resource.TestCheckResourceAttr("portainer_backup_azure_settings.a", "last_run_failed", "false"),
					resource.TestCheckNoResourceAttr("portainer_backup_azure_settings.a", "service_url"),
					func(*terraform.State) error {
						payload := gsbBody(t, mock, "POST", "/backup/azure/settings")
						for k, want := range map[string]interface{}{
							"authMethod": "servicePrincipalSecret", "storageAccountName": "acmebackups",
							"containerName": "portainer", "tenantId": "tenant-1", "clientId": "client-1",
							"clientSecret": "secret-1", "cronRule": "0 2 * * *", "password": "archive-pass",
						} {
							if payload[k] != want {
								return fmt.Errorf("payload.%s: expected %v, got %v", k, want, payload[k])
							}
						}
						for _, absent := range []string{"storageAccountKey", "clientCertificate", "managedIdentityClientId", "serviceUrl"} {
							if _, present := payload[absent]; present {
								return fmt.Errorf("%s must be omitted when not configured, sending it empty would clear it", absent)
							}
						}
						return nil
					},
				),
			},
			{
				// Portainer echoes the stored secret; it must not land in state.
				PreConfig: func() {
					fake.set("clientSecret", "secret-from-server")
					fake.mu.Lock()
					fake.hidden = nil
					fake.mu.Unlock()
				},
				Config: mockProviderConfig(mock) + azureSettingsConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_backup_azure_settings.a", "client_secret", "secret-1"),
					resource.TestCheckResourceAttr("portainer_backup_azure_settings.a", "password", "archive-pass"),
				),
			},
			{
				ResourceName:            "portainer_backup_azure_settings.a",
				ImportState:             true,
				ImportStateId:           "anything",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"client_secret", "password"},
			},
		},
	})
}

// TestBackupAzureSettingsResource_ToleratesMissingStatus covers a fresh
// instance that has never run a scheduled backup: the status endpoint failing
// must not fail the read.
func TestBackupAzureSettingsResource_ToleratesMissingStatus(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	(&fakeEchoSettings{}).register(mock, "/backup/azure/settings")
	mock.On("GET", "/backup/azure/status", RespondString(http.StatusInternalServerError, "application/json", `{"message":"no run yet"}`))

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
resource "portainer_backup_azure_settings" "a" {
  auth_method          = "managedIdentity"
  storage_account_name = "acme"
  container_name       = "p"
}`,
			Check: resource.TestCheckNoResourceAttr("portainer_backup_azure_settings.a", "last_run_failed"),
		}},
	})
}

// TestBackupAzureResources_WriteOnlySecrets: the write-only values reach
// Portainer from each Azure resource.
func TestBackupAzureResources_WriteOnlySecrets(t *testing.T) {
	t.Parallel()
	secrets := `
  storage_account_key_wo                 = "account-key"
  storage_account_key_wo_version         = 1
  client_secret_wo                       = "client-secret"
  client_secret_wo_version               = 1
  client_certificate_wo                  = "client-cert"
  client_certificate_wo_version          = 1
  client_certificate_password_wo         = "cert-pass"
  client_certificate_password_wo_version = 1
  password_wo                            = "backup-pass"
  password_wo_version                    = 1`
	want := map[string]string{
		"storageAccountKey": "account-key", "clientSecret": "client-secret", "clientCertificate": "client-cert",
		"clientCertificatePassword": "cert-pass", "password": "backup-pass",
	}
	cases := []struct {
		name, typ, path, extra string
		want                   map[string]string
	}{
		{"execute", "portainer_backup_azure_execute", "/backup/azure/execute", "", want},
		{"settings", "portainer_backup_azure_settings", "/backup/azure/settings", "", want},
		{"restore", "portainer_backup_azure_restore", "/backup/azure/restore", `blob_name = "b.tar.gz"`, map[string]string{
			"StorageAccountKey": "account-key", "ClientSecret": "client-secret", "ClientCertificate": "client-cert",
			"ClientCertificatePassword": "cert-pass", "Password": "backup-pass",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := NewMockServer(t)
			if tc.name == "settings" {
				(&fakeEchoSettings{hidden: []string{"storageAccountKey", "clientSecret", "clientCertificate", "clientCertificatePassword", "password"}}).register(mock, tc.path)
			} else {
				mock.On("POST", tc.path, RespondJSON(http.StatusOK, map[string]interface{}{}))
			}
			mock.On("GET", "/backup/azure/status", RespondJSON(http.StatusOK, map[string]interface{}{}))
			config := mockProviderConfig(mock) + fmt.Sprintf(`
resource %q "a" {
  auth_method          = "storageAccountKey"
  storage_account_name = "acct"
  container_name       = "backups"
  %s
  %s
}`, tc.typ, tc.extra, secrets)
			resource.UnitTest(t, resource.TestCase{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Steps: []resource.TestStep{
					{
						Config: config,
						Check: resource.ComposeTestCheckFunc(
							resource.TestCheckNoResourceAttr(tc.typ+".a", "storage_account_key"),
							resource.TestCheckNoResourceAttr(tc.typ+".a", "password"),
							func(*terraform.State) error {
								body := gsbBody(t, mock, "POST", tc.path)
								for key, value := range tc.want {
									if body[key] != value {
										return fmt.Errorf("%s: expected the write-only value %q, got %v", key, value, body[key])
									}
								}
								return nil
							},
						),
					},
					{Config: config, PlanOnly: true},
				},
			})
		})
	}
}

// TestBackupAzureExecuteResource_OmitsCronRule: the ad-hoc backup does not
// carry a schedule; it must back up once without rewriting the stored cron.
// A change of triggers runs it again.
func TestBackupAzureExecuteResource_OmitsCronRule(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/backup/azure/execute", RespondString(http.StatusNoContent, "", ""))
	config := func(run string) string {
		return mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_backup_azure_execute" "a" {
  auth_method          = "storageAccountKey"
  storage_account_name = "acmebackups"
  container_name       = "portainer"
  storage_account_key  = "key-1"
  triggers             = { run = %q }
}`, run)
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config("1"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestMatchResourceAttr("portainer_backup_azure_execute.a", "id", regexp.MustCompile(`^portainer-backup-azure-execute-\d+$`)),
					func(*terraform.State) error {
						payload := gsbBody(t, mock, "POST", "/backup/azure/execute")
						if _, present := payload["cronRule"]; present {
							return fmt.Errorf("an ad-hoc backup must not send a cron rule; it would rewrite the stored schedule")
						}
						if payload["storageAccountKey"] != "key-1" {
							return fmt.Errorf("storageAccountKey: got %v", payload["storageAccountKey"])
						}
						return nil
					},
				),
			},
			{
				Config: config("2"),
				Check: func(*terraform.State) error {
					if n := gsbCount(mock, "POST", "/backup/azure/execute"); n != 2 {
						return fmt.Errorf("a new trigger must run the backup again, got %d runs", n)
					}
					return nil
				},
			},
		},
	})
}

func localSettingsMock(t *testing.T) (*MockServer, *fakeEchoSettings) {
	mock := NewMockServer(t)
	fake := &fakeEchoSettings{}
	fake.register(mock, "/backup/local/settings")
	mock.On("GET", "/backup/local/status", RespondJSON(http.StatusOK, map[string]interface{}{
		"failed": true, "timestampUTC": "2026-09-14T03:00:00Z", "path": "/data/backups/latest.tar.gz",
	}))
	return mock, fake
}

// TestBackupLocalSettingsResource_RoundTrip covers the local destination,
// whose status object uses lowercase field names unlike Azure and S3.
func TestBackupLocalSettingsResource_RoundTrip(t *testing.T) {
	t.Parallel()
	mock, fake := localSettingsMock(t)
	config := mockProviderConfig(mock) + `
resource "portainer_backup_local_settings" "l" {
  cron_rule      = "0 3 * * *"
  retention_days = 14
}`

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_backup_local_settings.l", "id", "portainer-backup-local-settings"),
					resource.TestCheckResourceAttr("portainer_backup_local_settings.l", "last_run_failed", "true"),
					resource.TestCheckResourceAttr("portainer_backup_local_settings.l", "last_run_path", "/data/backups/latest.tar.gz"),
					func(*terraform.State) error {
						payload := gsbBody(t, mock, "POST", "/backup/local/settings")
						if payload["cronRule"] != "0 3 * * *" || payload["retentionDays"] != float64(14) {
							return fmt.Errorf("payload mismatch: %v", payload)
						}
						return nil
					},
				),
			},
			{
				PreConfig:          func() { fake.set("retentionDays", 30) },
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				ResourceName:      "portainer_backup_local_settings.l",
				ImportState:       true,
				ImportStateId:     "anything",
				ImportStateVerify: false,
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) != 1 || states[0].ID != "portainer-backup-local-settings" || states[0].Attributes["retention_days"] != "30" {
						return fmt.Errorf("unexpected import: %v", states)
					}
					return nil
				},
			},
			{
				// Unsetting everything sends empty values.
				Config: mockProviderConfig(mock) + `resource "portainer_backup_local_settings" "l" {}`,
				Check: func(*terraform.State) error {
					payload := gsbBody(t, mock, "POST", "/backup/local/settings")
					if payload["cronRule"] != "" || payload["retentionDays"] != float64(0) {
						return fmt.Errorf("payload mismatch: %v", payload)
					}
					return nil
				},
			},
		},
	})
}

// TestBackupLocalSettingsResource_Password covers the archive password
// Business Edition 3.1 added (required in FIPS mode): sent from password or
// its write-only twin, and left out of the payload when neither is set.
func TestBackupLocalSettingsResource_Password(t *testing.T) {
	t.Parallel()
	mock, _ := localSettingsMock(t)
	const res = "portainer_backup_local_settings.l"

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + `
resource "portainer_backup_local_settings" "l" {
  cron_rule = "0 3 * * *"
  password  = "plain-pass"
}`,
				Check: func(*terraform.State) error {
					if p := gsbBody(t, mock, "POST", "/backup/local/settings")["password"]; p != "plain-pass" {
						return fmt.Errorf("password: expected the configured value, got %v", p)
					}
					return nil
				},
			},
			{
				Config: mockProviderConfig(mock) + `
resource "portainer_backup_local_settings" "l" {
  cron_rule           = "0 3 * * *"
  password_wo         = "wo-pass"
  password_wo_version = 1
}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr(res, "password"),
					resource.TestCheckNoResourceAttr(res, "password_wo"),
					func(*terraform.State) error {
						if p := gsbBody(t, mock, "POST", "/backup/local/settings")["password"]; p != "wo-pass" {
							return fmt.Errorf("password: expected the write-only value, got %v", p)
						}
						return nil
					},
				),
			},
			{
				Config: mockProviderConfig(mock) + `
resource "portainer_backup_local_settings" "l" {
  cron_rule = "0 3 * * *"
}`,
				Check: func(*terraform.State) error {
					if p, ok := gsbBody(t, mock, "POST", "/backup/local/settings")["password"]; ok {
						return fmt.Errorf("password must be omitted when not configured, got %v", p)
					}
					return nil
				},
			},
		},
	})
}

func TestBackupLocalSettingsResource_NegativeRetentionRejected(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `resource "portainer_backup_local_settings" "l" { retention_days = -1 }`,
			ExpectError: regexp.MustCompile(`at least 0`),
		}},
	})
}

// TestBackupLocalRunResource_TriggersAndReadsStatus covers the on-demand
// local backup.
func TestBackupLocalRunResource_TriggersAndReadsStatus(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/backup/local/run", RespondString(http.StatusNoContent, "", ""))
	mock.On("GET", "/backup/local/status", RespondJSON(http.StatusOK, map[string]interface{}{
		"failed": false, "timestampUTC": "2026-09-15T10:00:00Z", "path": "/data/backups/adhoc.tar.gz",
	}))

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `resource "portainer_backup_local_run" "r" {}`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("portainer_backup_local_run.r", "path", "/data/backups/adhoc.tar.gz"),
				resource.TestCheckResourceAttr("portainer_backup_local_run.r", "failed", "false"),
				resource.TestCheckResourceAttr("portainer_backup_local_run.r", "timestamp", "2026-09-15T10:00:00Z"),
				resource.TestMatchResourceAttr("portainer_backup_local_run.r", "id", regexp.MustCompile(`^portainer-backup-local-run-\d+$`)),
				func(*terraform.State) error {
					if gsbCount(mock, "POST", "/backup/local/run") != 1 {
						return fmt.Errorf("expected POST /backup/local/run")
					}
					return nil
				},
			),
		}},
	})
}

// TestBackupRestoreResources_UsePascalCasePayloads is the trap this block is
// most likely to fall into: the restore endpoints capitalise their fields
// while the settings endpoints beside them do not.
func TestBackupRestoreResources_UsePascalCasePayloads(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/backup/s3/restore", RespondString(http.StatusNoContent, "", ""))
	mock.On("POST", "/backup/azure/restore", RespondString(http.StatusNoContent, "", ""))

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
resource "portainer_backup_s3_restore" "s" {
  bucket_name       = "acme-backups"
  filename          = "portainer-2026-09-15.tar.gz"
  region            = "eu-central-1"
  access_key_id     = "AKIA"
  secret_access_key = "secret"
}
resource "portainer_backup_azure_restore" "a" {
  auth_method          = "storageAccountKey"
  storage_account_name = "acmebackups"
  container_name       = "portainer"
  blob_name            = "portainer-2026-09-15.tar.gz"
  storage_account_key  = "key-1"
}`,
			Check: func(*terraform.State) error {
				s3 := gsbBody(t, mock, "POST", "/backup/s3/restore")
				for _, k := range []string{"BucketName", "Filename", "Region", "AccessKeyID", "SecretAccessKey"} {
					if _, ok := s3[k]; !ok {
						return fmt.Errorf("S3 restore payload must use PascalCase; %q is missing (got %v)", k, s3)
					}
				}
				for _, k := range []string{"S3CompatibleHost", "Password"} {
					if _, ok := s3[k]; ok {
						return fmt.Errorf("S3 restore payload must omit unset %q", k)
					}
				}
				az := gsbBody(t, mock, "POST", "/backup/azure/restore")
				for _, k := range []string{"AuthMethod", "StorageAccountName", "ContainerName", "BlobName", "StorageAccountKey"} {
					if _, ok := az[k]; !ok {
						return fmt.Errorf("Azure restore payload must use PascalCase; %q is missing (got %v)", k, az)
					}
				}
				return nil
			},
		}},
	})
}

func TestBackupS3RestoreResource_WriteOnlySecrets(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/backup/s3/restore", RespondJSON(http.StatusOK, map[string]interface{}{}))

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
resource "portainer_backup_s3_restore" "s" {
  bucket_name                  = "b"
  filename                     = "f.tar.gz"
  region                       = "eu-1"
  access_key_id                = "AKIA"
  secret_access_key_wo         = "s3-secret"
  secret_access_key_wo_version = 1
  password_wo                  = "backup-pass"
  password_wo_version          = 1
}`,
			Check: func(*terraform.State) error {
				body := gsbBody(t, mock, "POST", "/backup/s3/restore")
				if body["SecretAccessKey"] != "s3-secret" || body["Password"] != "backup-pass" {
					return fmt.Errorf("expected the write-only values, got %v", body)
				}
				return nil
			},
		}},
	})
}

// TestRestoreResource_SendsDecodedArchive: the archive round-trips. The
// configuration supplies base64, Portainer's FileContent is []byte, and
// encoding/json re-encodes it, so what arrives must equal what was configured.
func TestRestoreResource_SendsDecodedArchive(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/restore", RespondString(http.StatusNoContent, "", ""))
	archive := []byte("fake tar.gz bytes")

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_restore" "r" {
  file_content_base64 = %q
  file_name           = "backup.tar.gz"
  password            = "secret"
}`, base64.StdEncoding.EncodeToString(archive)),
				Check: func(*terraform.State) error {
					payload := gsbBody(t, mock, "POST", "/restore")
					if payload["FileName"] != "backup.tar.gz" || payload["Password"] != "secret" {
						return fmt.Errorf("payload mismatch: %v", payload)
					}
					got, err := base64.StdEncoding.DecodeString(payload["FileContent"].(string))
					if err != nil || string(got) != string(archive) {
						return fmt.Errorf("archive mismatch: %q (%w)", got, err)
					}
					if h := mock.FindRequest("POST", "/restore").Headers.Get("X-Setup-Token"); h != "" {
						return fmt.Errorf("no setup token is configured, got X-Setup-Token %q", h)
					}
					return nil
				},
			},
			{
				// file_name defaults to portainer-backup.tar.gz.
				Config: mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_restore" "r" {
  file_content_base64 = %q
}`, base64.StdEncoding.EncodeToString(archive)),
				Check: resource.TestCheckResourceAttr("portainer_restore.r", "file_name", "portainer-backup.tar.gz"),
			},
		},
	})
}

// TestRestoreResource_RejectsInvalidBase64: bad input fails before any
// request is made, rather than sending an empty archive to a restore endpoint.
func TestRestoreResource_RejectsInvalidBase64(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/restore", RespondString(http.StatusNoContent, "", ""))

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `resource "portainer_restore" "r" { file_content_base64 = "not base64!!" }`,
			ExpectError: regexp.MustCompile(`not valid base64`),
		}},
	})
	if mock.FindRequest("POST", "/restore") != nil {
		t.Error("no request should be made when the input cannot be decoded")
	}
}

// TestRestoreResource_WriteOnly: the archive, password and setup token come
// from the write-only attributes; the token is a header next to the API key.
func TestRestoreResource_WriteOnly(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/restore", RespondJSON(http.StatusOK, map[string]interface{}{}))
	archive := base64.StdEncoding.EncodeToString([]byte("archive"))

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_restore" "r" {
  file_content_base64_wo         = %q
  file_content_base64_wo_version = 1
  password_wo                    = "backup-pass"
  password_wo_version            = 1
  setup_token_wo                 = "setup-token"
  setup_token_wo_version         = 1
}`, archive),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckNoResourceAttr("portainer_restore.r", "file_content_base64"),
				func(*terraform.State) error {
					req := mock.FindRequest("POST", "/restore")
					if got := req.Headers.Get("X-Setup-Token"); got != "setup-token" {
						return fmt.Errorf("X-Setup-Token: expected the write-only value, got %q", got)
					}
					// The setup token must not displace the API key.
					if req.Headers.Get("X-API-Key") == "" {
						return fmt.Errorf("the API key header must still be sent alongside the setup token")
					}
					body := gsbBody(t, mock, "POST", "/restore")
					if body["Password"] != "backup-pass" || body["FileContent"] != archive {
						return fmt.Errorf("expected the write-only archive and password, got %v", body)
					}
					return nil
				},
			),
		}},
	})
}

// TestBackupAzureConnectionDataSource_ReportsOutcome covers the pre-flight
// check, whose answer is the status code rather than a body; by default a
// failed check fails the data source, since it exists to gate a configuration.
func TestBackupAzureConnectionDataSource_ReportsOutcome(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/backup/azure/test", RespondString(http.StatusBadRequest, "application/json", `{"message":"container not found"}`))
	config := func(extra string) string {
		return mockProviderConfig(mock) + fmt.Sprintf(`
data "portainer_backup_azure_connection" "c" {
  auth_method          = "storageAccountKey"
  storage_account_name = "acme"
  container_name       = "missing"
  storage_account_key  = "key"
  %s
}`, extra)
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config("fail_on_error = false"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.portainer_backup_azure_connection.c", "success", "false"),
					resource.TestMatchResourceAttr("data.portainer_backup_azure_connection.c", "error", regexp.MustCompile("container not found")),
					resource.TestCheckResourceAttr("data.portainer_backup_azure_connection.c", "id", "azure-backup-test-acme/missing"),
					func(*terraform.State) error {
						body := gsbBody(t, mock, "POST", "/backup/azure/test")
						if body["storageAccountKey"] != "key" || body["authMethod"] != "storageAccountKey" {
							return fmt.Errorf("payload mismatch: %v", body)
						}
						return nil
					},
				),
			},
			{
				Config:      config(""),
				ExpectError: regexp.MustCompile(`the Azure Blob backup connection test failed`),
			},
		},
	})
}

func TestBackupAzureConnectionDataSource_Success(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/backup/azure/test", RespondString(http.StatusNoContent, "", ""))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
data "portainer_backup_azure_connection" "c" {
  auth_method          = "managedIdentity"
  storage_account_name = "acme"
  container_name       = "ok"
}`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("data.portainer_backup_azure_connection.c", "success", "true"),
				resource.TestCheckResourceAttr("data.portainer_backup_azure_connection.c", "error", ""),
				resource.TestCheckResourceAttr("data.portainer_backup_azure_connection.c", "fail_on_error", "true"),
			),
		}},
	})
}

func TestBackupAzureSettingsResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	newMock := func(t *testing.T) *MockServer { m, _ := azureSettingsMock(t); return m }
	gsbUpgradeTest(t, newMock,
		azureSettingsConfig,
		`
resource "portainer_backup_azure_settings" "a" {
  auth_method          = "managedIdentity"
  storage_account_name = "acme"
  container_name       = "p"
}`,
		`
resource "portainer_backup_azure_settings" "a" {
  auth_method                    = "storageAccountKey"
  storage_account_name           = "acme"
  container_name                 = "p"
  storage_account_key_wo         = "k"
  storage_account_key_wo_version = 1
  service_url                    = "https://acme.blob.core.windows.net"
}`,
	)
}

func TestBackupAzureExecuteResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	newMock := func(t *testing.T) *MockServer {
		m := NewMockServer(t)
		m.On("POST", "/backup/azure/execute", RespondString(http.StatusNoContent, "", ""))
		return m
	}
	gsbUpgradeTest(t, newMock,
		`
resource "portainer_backup_azure_execute" "a" {
  auth_method          = "storageAccountKey"
  storage_account_name = "acme"
  container_name       = "p"
  storage_account_key  = "k"
  triggers             = { run = "1" }
}`,
		`
resource "portainer_backup_azure_execute" "a" {
  auth_method              = "servicePrincipalSecret"
  storage_account_name     = "acme"
  container_name           = "p"
  tenant_id                = "t"
  client_id                = "c"
  client_secret_wo         = "s"
  client_secret_wo_version = 1
}`,
	)
}

func TestBackupAzureRestoreResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	newMock := func(t *testing.T) *MockServer {
		m := NewMockServer(t)
		m.On("POST", "/backup/azure/restore", RespondString(http.StatusNoContent, "", ""))
		return m
	}
	gsbUpgradeTest(t, newMock,
		`
resource "portainer_backup_azure_restore" "a" {
  auth_method          = "storageAccountKey"
  storage_account_name = "acme"
  container_name       = "p"
  blob_name            = "b.tar.gz"
  storage_account_key  = "k"
  password             = "p"
}`,
		`
resource "portainer_backup_azure_restore" "a" {
  auth_method          = "managedIdentity"
  storage_account_name = "acme"
  container_name       = "p"
  blob_name            = "b.tar.gz"
  password_wo          = "p"
  password_wo_version  = 1
}`,
	)
}

func TestBackupLocalSettingsResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	newMock := func(t *testing.T) *MockServer { m, _ := localSettingsMock(t); return m }
	gsbUpgradeTest(t, newMock,
		`resource "portainer_backup_local_settings" "l" {
  cron_rule      = "0 3 * * *"
  retention_days = 14
}`,
		`resource "portainer_backup_local_settings" "l" {}`,
	)
}

func TestBackupLocalRunResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	newMock := func(t *testing.T) *MockServer {
		m := NewMockServer(t)
		m.On("POST", "/backup/local/run", RespondString(http.StatusNoContent, "", ""))
		m.On("GET", "/backup/local/status", RespondJSON(http.StatusOK, map[string]interface{}{"failed": false, "timestampUTC": "t", "path": "p"}))
		return m
	}
	gsbUpgradeTest(t, newMock,
		`resource "portainer_backup_local_run" "r" {}`,
		`resource "portainer_backup_local_run" "r" {
  triggers = { at = "x" }
}`,
	)
}

func TestBackupS3RestoreResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	newMock := func(t *testing.T) *MockServer {
		m := NewMockServer(t)
		m.On("POST", "/backup/s3/restore", RespondString(http.StatusNoContent, "", ""))
		return m
	}
	gsbUpgradeTest(t, newMock,
		`
resource "portainer_backup_s3_restore" "s" {
  bucket_name       = "b"
  filename          = "f.tar.gz"
  region            = "eu-1"
  access_key_id     = "AKIA"
  secret_access_key = "secret"
}`,
		`
resource "portainer_backup_s3_restore" "s" {
  bucket_name                  = "b"
  filename                     = "f.tar.gz"
  region                       = "eu-1"
  access_key_id                = "AKIA"
  secret_access_key_wo         = "s"
  secret_access_key_wo_version = 1
  s3_compatible_host           = "minio:9000"
  password                     = "p"
}`,
	)
}

func TestRestoreResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	newMock := func(t *testing.T) *MockServer {
		m := NewMockServer(t)
		m.On("POST", "/restore", RespondString(http.StatusNoContent, "", ""))
		return m
	}
	archive := base64.StdEncoding.EncodeToString([]byte("archive"))
	gsbUpgradeTest(t, newMock,
		fmt.Sprintf(`resource "portainer_restore" "r" { file_content_base64 = %q }`, archive),
		fmt.Sprintf(`
resource "portainer_restore" "r" {
  file_content_base64_wo         = %q
  file_content_base64_wo_version = 1
  file_name                      = "x.tar.gz"
  password                       = "p"
  setup_token_wo                 = "t"
  setup_token_wo_version         = 1
}`, archive),
	)
}
