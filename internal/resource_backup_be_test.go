package internal

import (
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// =========================================================================
// Business Edition backup coverage. Portainer's backup API is split across
// three destinations with deliberately inconsistent JSON conventions: the
// settings payloads are camelCase, the restore payloads are PascalCase, and the
// status objects differ per destination (Azure and S3 capitalise their fields,
// local does not). Getting any of those wrong is silent — the request is
// accepted and the fields simply arrive empty — so the tests below pin the wire
// format rather than just the happy path.
// =========================================================================

func azureSettingsResourceData(t *testing.T) (*schema.Resource, *schema.ResourceData) {
	t.Helper()
	r := resourceBackupAzureSettings()
	d := r.TestResourceData()
	_ = d.Set("auth_method", "servicePrincipalSecret")
	_ = d.Set("storage_account_name", "acmebackups")
	_ = d.Set("container_name", "portainer")
	_ = d.Set("tenant_id", "tenant-1")
	_ = d.Set("client_id", "client-1")
	_ = d.Set("client_secret", "secret-1")
	_ = d.Set("cron_rule", "0 2 * * *")
	_ = d.Set("password", "archive-pass")
	return r, d
}

// TestBackupAzureSettings_WritesCamelCasePayload pins the settings wire format
// and that unset optional fields are omitted rather than sent empty, which
// would clear a credential configured elsewhere.
func TestBackupAzureSettings_WritesCamelCasePayload(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/backup/azure/settings", RespondString(http.StatusNoContent, "", ""))
	mock.On("GET", "/backup/azure/settings", RespondJSON(http.StatusOK, map[string]interface{}{
		"authMethod": "servicePrincipalSecret", "storageAccountName": "acmebackups",
		"containerName": "portainer", "tenantId": "tenant-1", "clientId": "client-1",
		"cronRule": "0 2 * * *",
	}))
	mock.On("GET", "/backup/azure/status", RespondJSON(http.StatusOK, map[string]interface{}{
		"Failed": false, "TimestampUTC": "2026-09-15T02:00:00Z",
	}))

	r, d := azureSettingsResourceData(t)
	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	post := mock.FindRequest("POST", "/backup/azure/settings")
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	for k, want := range map[string]interface{}{
		"authMethod": "servicePrincipalSecret", "storageAccountName": "acmebackups",
		"containerName": "portainer", "tenantId": "tenant-1", "clientId": "client-1",
		"clientSecret": "secret-1", "cronRule": "0 2 * * *", "password": "archive-pass",
	} {
		if payload[k] != want {
			t.Errorf("payload.%s: expected %v, got %v", k, want, payload[k])
		}
	}
	for _, absent := range []string{"storageAccountKey", "clientCertificate", "managedIdentityClientId", "serviceUrl"} {
		if _, present := payload[absent]; present {
			t.Errorf("%s must be omitted when not configured, sending it empty would clear it", absent)
		}
	}

	if got := d.Get("last_run_timestamp"); got != "2026-09-15T02:00:00Z" {
		t.Errorf("last_run_timestamp: got %v", got)
	}
}

// TestBackupAzureSettings_ReadKeepsSecrets verifies the read does not overwrite
// configured credentials with whatever the server hands back.
func TestBackupAzureSettings_ReadKeepsSecrets(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/backup/azure/settings", RespondJSON(http.StatusOK, map[string]interface{}{
		"authMethod": "servicePrincipalSecret", "storageAccountName": "acmebackups",
		"containerName": "portainer",
		// Portainer echoes the stored secret; it must not land in state from here.
		"clientSecret": "secret-from-server", "password": "pass-from-server",
	}))
	mock.On("GET", "/backup/azure/status", RespondString(http.StatusNotFound, "application/json", `{}`))

	r, d := azureSettingsResourceData(t)
	d.SetId("portainer-backup-azure-settings")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("client_secret"); got != "secret-1" {
		t.Errorf("client_secret must stay as configured, got %v", got)
	}
	if got := d.Get("password"); got != "archive-pass" {
		t.Errorf("password must stay as configured, got %v", got)
	}
}

// TestBackupAzureSettings_ReadToleratesMissingStatus covers a fresh instance
// that has never run a scheduled backup: the status endpoint failing must not
// fail the read.
func TestBackupAzureSettings_ReadToleratesMissingStatus(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/backup/azure/settings", RespondJSON(http.StatusOK, map[string]interface{}{
		"authMethod": "managedIdentity", "storageAccountName": "acme", "containerName": "p",
	}))
	mock.On("GET", "/backup/azure/status",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"no run yet"}`))

	r, d := azureSettingsResourceData(t)
	d.SetId("portainer-backup-azure-settings")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("a missing status must not fail the read: %v", err)
	}
}

// TestBackupAzureExecute_OmitsCronRule verifies the ad-hoc backup does not
// carry a schedule: it must back up once without rewriting the stored cron.
func TestBackupAzureExecute_OmitsCronRule(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/backup/azure/execute", RespondString(http.StatusNoContent, "", ""))

	r := resourceBackupAzureExecute()
	d := r.TestResourceData()
	_ = d.Set("auth_method", "storageAccountKey")
	_ = d.Set("storage_account_name", "acmebackups")
	_ = d.Set("container_name", "portainer")
	_ = d.Set("storage_account_key", "key-1")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	post := mock.FindRequest("POST", "/backup/azure/execute")
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if _, present := payload["cronRule"]; present {
		t.Error("an ad-hoc backup must not send a cron rule; it would rewrite the stored schedule")
	}
	if payload["storageAccountKey"] != "key-1" {
		t.Errorf("storageAccountKey: got %v", payload["storageAccountKey"])
	}
}

// TestBackupLocalSettings_RoundTrip covers the local destination, whose status
// object uses lowercase field names unlike Azure and S3.
func TestBackupLocalSettings_RoundTrip(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/backup/local/settings", RespondString(http.StatusNoContent, "", ""))
	mock.On("GET", "/backup/local/settings", RespondJSON(http.StatusOK, map[string]interface{}{
		"cronRule": "0 3 * * *", "retentionDays": 14,
	}))
	mock.On("GET", "/backup/local/status", RespondJSON(http.StatusOK, map[string]interface{}{
		"failed": true, "timestampUTC": "2026-09-14T03:00:00Z", "path": "/data/backups/latest.tar.gz",
	}))

	r := resourceBackupLocalSettings()
	d := r.TestResourceData()
	_ = d.Set("cron_rule", "0 3 * * *")
	_ = d.Set("retention_days", 14)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	post := mock.FindRequest("POST", "/backup/local/settings")
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if payload["cronRule"] != "0 3 * * *" || payload["retentionDays"] != float64(14) {
		t.Errorf("payload mismatch: %v", payload)
	}
	if got := d.Get("last_run_failed"); got != true {
		t.Errorf("last_run_failed: expected the lowercase status field to be read, got %v", got)
	}
	if got := d.Get("last_run_path"); got != "/data/backups/latest.tar.gz" {
		t.Errorf("last_run_path: got %v", got)
	}
}

// TestBackupLocalRun_TriggersAndReadsStatus covers the on-demand local backup.
func TestBackupLocalRun_TriggersAndReadsStatus(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/backup/local/run", RespondString(http.StatusNoContent, "", ""))
	mock.On("GET", "/backup/local/status", RespondJSON(http.StatusOK, map[string]interface{}{
		"failed": false, "timestampUTC": "2026-09-15T10:00:00Z", "path": "/data/backups/adhoc.tar.gz",
	}))

	r := resourceBackupLocalRun()
	d := r.TestResourceData()

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if mock.FindRequest("POST", "/backup/local/run") == nil {
		t.Fatal("expected POST /backup/local/run")
	}
	if got := d.Get("path"); got != "/data/backups/adhoc.tar.gz" {
		t.Errorf("path: got %v", got)
	}
	if !strings.HasPrefix(d.Id(), "portainer-backup-local-run-") {
		t.Errorf("unexpected ID: %q", d.Id())
	}
}

// TestBackupRestore_UsesPascalCasePayloads is the trap this block is most
// likely to fall into: the restore endpoints capitalise their fields while the
// settings endpoints beside them do not.
func TestBackupRestore_UsesPascalCasePayloads(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/backup/s3/restore", RespondString(http.StatusNoContent, "", ""))
	mock.On("POST", "/backup/azure/restore", RespondString(http.StatusNoContent, "", ""))

	s3 := resourceBackupS3Restore()
	ds3 := s3.TestResourceData()
	_ = ds3.Set("bucket_name", "acme-backups")
	_ = ds3.Set("filename", "portainer-2026-09-15.tar.gz")
	_ = ds3.Set("region", "eu-central-1")
	_ = ds3.Set("access_key_id", "AKIA")
	_ = ds3.Set("secret_access_key", "secret")
	if err := rcCreate(s3, ds3, mock.Client()); err != nil {
		t.Fatalf("S3 restore failed: %v", err)
	}
	var s3payload map[string]interface{}
	if err := mock.FindRequest("POST", "/backup/s3/restore").DecodeJSON(&s3payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	for _, k := range []string{"BucketName", "Filename", "Region", "AccessKeyID", "SecretAccessKey"} {
		if _, ok := s3payload[k]; !ok {
			t.Errorf("S3 restore payload must use PascalCase; %q is missing (got keys %v)", k, keysOf(s3payload))
		}
	}

	az := resourceBackupAzureRestore()
	daz := az.TestResourceData()
	_ = daz.Set("auth_method", "storageAccountKey")
	_ = daz.Set("storage_account_name", "acmebackups")
	_ = daz.Set("container_name", "portainer")
	_ = daz.Set("blob_name", "portainer-2026-09-15.tar.gz")
	_ = daz.Set("storage_account_key", "key-1")
	if err := rcCreate(az, daz, mock.Client()); err != nil {
		t.Fatalf("Azure restore failed: %v", err)
	}
	var azpayload map[string]interface{}
	if err := mock.FindRequest("POST", "/backup/azure/restore").DecodeJSON(&azpayload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	for _, k := range []string{"AuthMethod", "StorageAccountName", "ContainerName", "BlobName", "StorageAccountKey"} {
		if _, ok := azpayload[k]; !ok {
			t.Errorf("Azure restore payload must use PascalCase; %q is missing (got keys %v)", k, keysOf(azpayload))
		}
	}
}

// TestBackupAzureConnection_ReportsOutcome covers the pre-flight check, whose
// answer is the status code rather than a body.
func TestBackupAzureConnection_ReportsOutcome(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/backup/azure/test",
		RespondString(http.StatusBadRequest, "application/json", `{"message":"container not found"}`))

	ds := dataSourceBackupAzureConnection()
	d := ds.TestResourceData()
	_ = d.Set("auth_method", "storageAccountKey")
	_ = d.Set("storage_account_name", "acme")
	_ = d.Set("container_name", "missing")
	_ = d.Set("storage_account_key", "key")
	_ = d.Set("fail_on_error", false)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("with fail_on_error=false the outcome must be reported: %v", err)
	}
	if got := d.Get("success"); got != false {
		t.Errorf("success: expected false, got %v", got)
	}
	if !strings.Contains(d.Get("error").(string), "container not found") {
		t.Errorf("error should carry the server's message, got %q", d.Get("error"))
	}

	// The default is to fail, since this exists to gate a configuration.
	if got := dataSourceBackupAzureConnection().Schema["fail_on_error"].Default; got != true {
		t.Errorf("fail_on_error should default to true, got %v", got)
	}
}

func keysOf(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
