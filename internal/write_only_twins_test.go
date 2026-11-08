package internal

import (
	"encoding/base64"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// The write-only twins of the backup, restore, registry and image secrets:
// each test checks the write-only value reaches Portainer while the plaintext
// attribute stays empty in state.

func TestWriteOnly_Backup(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/backup", RespondString(http.StatusOK, "application/gzip", "archive"))

	r := resourceBackup()
	d := dataWithRawConfig(r, map[string]string{
		"output_path": filepath.Join(t.TempDir(), "backup.tar.gz"), "password_wo_version": "1",
	}, map[string]cty.Value{"password_wo": cty.StringVal("backup-pass")})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if got := decodeBody(t, mock, "POST", "/backup")["password"]; got != "backup-pass" {
		t.Errorf("password: expected the write-only value, got %v", got)
	}
	if d.Get("password") != "" {
		t.Error("password must stay out of state")
	}
}

func TestWriteOnly_BackupS3(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/backup/s3/execute", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceBackupS3()
	d := dataWithRawConfig(r, map[string]string{
		"access_key_id": "AKIA", "bucket_name": "b", "region": "eu-1", "s3_compatible_host": "s3.example.com",
		"secret_access_key_wo_version": "1", "password_wo_version": "1",
	}, map[string]cty.Value{
		"secret_access_key_wo": cty.StringVal("s3-secret"),
		"password_wo":          cty.StringVal("backup-pass"),
	})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	body := decodeBody(t, mock, "POST", "/backup/s3/execute")
	if body["secretAccessKey"] != "s3-secret" || body["password"] != "backup-pass" {
		t.Errorf("expected the write-only values, got %v", body)
	}
}

// TestWriteOnly_BackupS3ReadKeepsSecretsOut covers Read, which copies the
// S3 secrets back from Portainer: with the write-only twin in use, that would
// store them in state and plan a replacement on every run.
func TestWriteOnly_BackupS3ReadKeepsSecretsOut(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/backup/s3/settings", RespondJSON(http.StatusOK, map[string]interface{}{
		"accessKeyID": "AKIA", "secretAccessKey": "s3-secret", "password": "backup-pass", "bucketName": "b",
	}))
	mock.On("GET", "/backup/s3/status", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceBackupS3()
	d := r.TestResourceData()
	d.SetId("portainer_backup_s3")
	_ = d.Set("secret_access_key_wo_version", 1)
	_ = d.Set("password_wo_version", 1)

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Get("secret_access_key") != "" || d.Get("password") != "" {
		t.Errorf("secrets given write-only must not be read into state, got %v / %v", d.Get("secret_access_key"), d.Get("password"))
	}
	if d.Get("access_key_id") != "AKIA" {
		t.Errorf("the other settings are still read, got %v", d.Get("access_key_id"))
	}
}

func TestWriteOnly_BackupAzure(t *testing.T) {
	azureSecrets := map[string]cty.Value{
		"storage_account_key_wo":         cty.StringVal("account-key"),
		"client_secret_wo":               cty.StringVal("client-secret"),
		"client_certificate_wo":          cty.StringVal("client-cert"),
		"client_certificate_password_wo": cty.StringVal("cert-pass"),
		"password_wo":                    cty.StringVal("backup-pass"),
	}
	attrs := func(extra map[string]string) map[string]string {
		a := map[string]string{"auth_method": "storage_account_key", "storage_account_name": "acct", "container_name": "backups"}
		for _, k := range []string{"storage_account_key", "client_secret", "client_certificate", "client_certificate_password", "password"} {
			a[k+"_wo_version"] = "1"
		}
		for k, v := range extra {
			a[k] = v
		}
		return a
	}
	want := map[string]string{
		"storageAccountKey": "account-key", "clientSecret": "client-secret", "clientCertificate": "client-cert",
		"clientCertificatePassword": "cert-pass", "password": "backup-pass",
	}
	cases := []struct {
		name     string
		resource *schema.Resource
		path     string
		attrs    map[string]string
		want     map[string]string
	}{
		{"execute", resourceBackupAzureExecute(), "/backup/azure/execute", attrs(nil), want},
		{"settings", resourceBackupAzureSettings(), "/backup/azure/settings", attrs(nil), want},
		{"restore", resourceBackupAzureRestore(), "/backup/azure/restore", attrs(map[string]string{"blob_name": "b.tar.gz"}), map[string]string{
			"StorageAccountKey": "account-key", "ClientSecret": "client-secret", "ClientCertificate": "client-cert",
			"ClientCertificatePassword": "cert-pass", "Password": "backup-pass",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := NewMockServer(t)
			mock.On("POST", tc.path, RespondJSON(http.StatusOK, map[string]interface{}{}))
			mock.On("GET", "/backup/azure/settings", RespondJSON(http.StatusOK, map[string]interface{}{}))
			mock.On("GET", "/backup/azure/status", RespondJSON(http.StatusOK, map[string]interface{}{}))

			d := dataWithRawConfig(tc.resource, tc.attrs, azureSecrets)
			if err := rcCreate(tc.resource, d, mock.Client()); err != nil {
				t.Fatalf("Create failed: %v", err)
			}
			body := decodeBody(t, mock, "POST", tc.path)
			for key, value := range tc.want {
				if body[key] != value {
					t.Errorf("%s: expected the write-only value %q, got %v", key, value, body[key])
				}
			}
		})
	}
}

func TestWriteOnly_BackupS3Restore(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/backup/s3/restore", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceBackupS3Restore()
	d := dataWithRawConfig(r, map[string]string{
		"bucket_name": "b", "filename": "f.tar.gz", "region": "eu-1", "access_key_id": "AKIA",
		"secret_access_key_wo_version": "1", "password_wo_version": "1",
	}, map[string]cty.Value{
		"secret_access_key_wo": cty.StringVal("s3-secret"),
		"password_wo":          cty.StringVal("backup-pass"),
	})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	body := decodeBody(t, mock, "POST", "/backup/s3/restore")
	if body["SecretAccessKey"] != "s3-secret" || body["Password"] != "backup-pass" {
		t.Errorf("expected the write-only values, got %v", body)
	}
}

func TestWriteOnly_Restore(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/restore", RespondJSON(http.StatusOK, map[string]interface{}{}))

	archive := base64.StdEncoding.EncodeToString([]byte("archive"))
	r := resourceRestore()
	d := dataWithRawConfig(r, map[string]string{
		"file_name":                      "portainer-backup.tar.gz",
		"file_content_base64_wo_version": "1", "password_wo_version": "1", "setup_token_wo_version": "1",
	}, map[string]cty.Value{
		"file_content_base64_wo": cty.StringVal(archive),
		"password_wo":            cty.StringVal("backup-pass"),
		"setup_token_wo":         cty.StringVal("setup-token"),
	})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	req := mock.FindRequest("POST", "/restore")
	if got := req.Headers.Get("X-Setup-Token"); got != "setup-token" {
		t.Errorf("X-Setup-Token: expected the write-only value, got %q", got)
	}
	body := decodeBody(t, mock, "POST", "/restore")
	if body["Password"] != "backup-pass" || body["FileContent"] != archive {
		t.Errorf("expected the write-only archive and password, got %v", body)
	}
}

func TestWriteOnly_RegistryConfigure(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/registries/1/configure", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceRegistryConfigure()
	d := dataWithRawConfig(r, map[string]string{
		"registry_id": "1", "authentication": "true", "username": "user", "tls": "true",
		"password_wo_version": "1", "tls_key_wo_version": "1",
	}, map[string]cty.Value{
		"password_wo": cty.StringVal("registry-pass"),
		"tls_key_wo":  cty.StringVal("PEM KEY"),
	})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	body := decodeBody(t, mock, "POST", "/registries/1/configure")
	if body["Password"] != "registry-pass" {
		t.Errorf("Password: expected the write-only value, got %v", body["Password"])
	}
	if body["TLSKeyFile"] != base64.StdEncoding.EncodeToString([]byte("PEM KEY")) {
		t.Errorf("TLSKeyFile: expected the write-only key, got %v", body["TLSKeyFile"])
	}
}

func TestWriteOnly_DockerImageRegistryAuth(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/endpoints/1/docker/images/create", RespondString(http.StatusOK, "application/json", `{"status":"done"}`))

	r := resourceDockerImage()
	d := dataWithRawConfig(r, map[string]string{
		"endpoint_id": "1", "image": "registry.example.com/app:1", "registry_auth_wo_version": "1",
	}, map[string]cty.Value{"registry_auth_wo": cty.StringVal("user:pass")})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	header := mock.FindRequest("POST", "/endpoints/1/docker/images/create").Headers.Get("X-Registry-Auth")
	decoded, _ := base64.StdEncoding.DecodeString(header)
	if !strings.Contains(string(decoded), `"password":"pass"`) || !strings.Contains(string(decoded), `"username":"user"`) {
		t.Errorf("X-Registry-Auth: expected the write-only credentials, got %s", decoded)
	}
}

func TestWriteOnly_LDAPSettingsTLSKey(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/settings", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("PUT", "/settings", RespondJSON(http.StatusOK, map[string]interface{}{}))

	// The state still holds the key the configuration had in tls_config
	// before moving to tls_key_wo; the write-only key has to win and the old
	// one has to leave the state.
	r := resourceLDAPSettings()
	d := dataWithRawConfig(r, map[string]string{
		"reader_dn": "cn=reader", "url": "ldap.example.com:636",
		"tls_config.#": "1", "tls_config.0.tls": "true", "tls_config.0.tls_key": "old-key",
		"tls_key_wo_version": "1",
	}, map[string]cty.Value{"tls_key_wo": cty.StringVal("new-key")})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	ldap, _ := decodeBody(t, mock, "PUT", "/settings")["ldapsettings"].(map[string]interface{})
	tls, _ := ldap["TLSConfig"].(map[string]interface{})
	if tls["TLSKey"] != "new-key" {
		t.Errorf("TLSKey: expected the write-only key, got %v", tls["TLSKey"])
	}
	if got := d.Get("tls_config.0.tls_key"); got != "" {
		t.Errorf("tls_config.tls_key: expected the old key cleared from state, got %v", got)
	}
}

func TestWriteOnly_LicenseKey(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/licenses/add", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceLicenses()
	d := dataWithRawConfig(r, map[string]string{"key_wo_version": "1"},
		map[string]cty.Value{"key_wo": cty.StringVal(testLicenseKey)})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if got := decodeBody(t, mock, "POST", "/licenses/add")["key"]; got != testLicenseKey {
		t.Errorf("key: expected the write-only value, got %v", got)
	}
	if d.Id() != sha256Hex(testLicenseKey) {
		t.Errorf("expected the ID to be the key's SHA-256, got %q", d.Id())
	}
	if d.Get("key") != "" {
		t.Error("key must stay out of state")
	}
}

// TestWriteOnly_ExactlyOneOfRequiredSecret checks a secret that was required
// stays required: either it or its write-only twin has to be configured.
func TestWriteOnly_ExactlyOneOfRequiredSecret(t *testing.T) {
	r := resourceBackup()
	cases := []struct {
		name    string
		config  map[string]interface{}
		wantErr bool
	}{
		{"plaintext", map[string]interface{}{"output_path": "/tmp/b", "password": "p"}, false},
		{"write-only", map[string]interface{}{"output_path": "/tmp/b", "password_wo": "p", "password_wo_version": 1}, false},
		{"neither", map[string]interface{}{"output_path": "/tmp/b"}, true},
		{"both", map[string]interface{}{"output_path": "/tmp/b", "password": "p", "password_wo": "p", "password_wo_version": 1}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diags := r.Validate(terraform.NewResourceConfigRaw(tc.config))
			if diags.HasError() != tc.wantErr {
				t.Errorf("expected error %v, got %v", tc.wantErr, diags)
			}
		})
	}
}
