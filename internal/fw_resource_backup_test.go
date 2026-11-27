package internal

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func backupMock(t *testing.T, archive []byte) *MockServer {
	mock := NewMockServer(t)
	mock.On("POST", "/backup", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(archive)
	})
	return mock
}

// TestBackupResource_WritesArchive: the password is posted to /backup and the
// streamed response lands at output_path.
func TestBackupResource_WritesArchive(t *testing.T) {
	t.Parallel()
	expected := []byte("\x1f\x8b\x08fake-tarball-bytes")
	mock := backupMock(t, expected)
	outPath := filepath.Join(t.TempDir(), "backup.tar.gz")

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_backup" "b" {
  password    = "topsecret"
  output_path = %q
}`, outPath),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttrSet("portainer_backup.b", "id"),
				func(*terraform.State) error {
					got, err := os.ReadFile(outPath)
					if err != nil {
						return err
					}
					if string(got) != string(expected) {
						return fmt.Errorf("backup file contents mismatch: got %q want %q", got, expected)
					}
					if p := gsbBody(t, mock, "POST", "/backup")["password"]; p != "topsecret" {
						return fmt.Errorf("payload.password: got %v", p)
					}
					return nil
				},
			),
		}},
	})
}

// TestBackupResource_HTTPError: a server error is surfaced and no file is
// written.
func TestBackupResource_HTTPError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/backup", RespondString(http.StatusInternalServerError, "application/json", `{"message":"backup failed"}`))
	outPath := filepath.Join(t.TempDir(), "should-not-exist.tar.gz")

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_backup" "b" {
  password    = "x"
  output_path = %q
}`, outPath),
			ExpectError: regexp.MustCompile(`backup failed`),
		}},
	})
	if _, err := os.Stat(outPath); err == nil {
		t.Error("expected output file to NOT exist when create fails before file write")
	}
}

// TestBackupResource_WriteOnlyPassword: the write-only value reaches
// Portainer while password stays out of state.
func TestBackupResource_WriteOnlyPassword(t *testing.T) {
	t.Parallel()
	mock := backupMock(t, []byte("archive"))
	outPath := filepath.Join(t.TempDir(), "backup.tar.gz")

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_backup" "b" {
  password_wo         = "backup-pass"
  password_wo_version = 1
  output_path         = %q
}`, outPath),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckNoResourceAttr("portainer_backup.b", "password"),
				resource.TestCheckNoResourceAttr("portainer_backup.b", "password_wo"),
				func(*terraform.State) error {
					if p := gsbBody(t, mock, "POST", "/backup")["password"]; p != "backup-pass" {
						return fmt.Errorf("password: expected the write-only value, got %v", p)
					}
					return nil
				},
			),
		}},
	})
}

// TestBackupResource_ExactlyOnePassword: a secret that was required stays
// required: either it or its write-only twin has to be configured.
func TestBackupResource_ExactlyOnePassword(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		attrs   string
		wantErr bool
	}{
		"plaintext":  {`password = "p"`, false},
		"write-only": {"password_wo = \"p\"\npassword_wo_version = 1", false},
		"neither":    {``, true},
		"both":       {"password = \"p\"\npassword_wo = \"p\"\npassword_wo_version = 1", true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			step := resource.TestStep{
				Config:   mockProviderConfig(mock) + fmt.Sprintf("resource \"portainer_backup\" \"b\" {\noutput_path = \"/tmp/b\"\n%s\n}", tc.attrs),
				PlanOnly: true, ExpectNonEmptyPlan: true,
			}
			if tc.wantErr {
				step.ExpectError = regexp.MustCompile(`Invalid Attribute Combination|Missing Attribute Configuration`)
			}
			resource.UnitTest(t, resource.TestCase{ProtoV5ProviderFactories: testProtoV5Factories, Steps: []resource.TestStep{step}})
		})
	}
}

func TestBackupResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	gsbUpgradeTest(t, func(t *testing.T) *MockServer { return backupMock(t, []byte("archive")) },
		fmt.Sprintf(`resource "portainer_backup" "b" {
  password    = "p"
  output_path = %q
}`, filepath.Join(dir, "a.tgz")),
		fmt.Sprintf(`resource "portainer_backup" "b" {
  password_wo         = "p"
  password_wo_version = 1
  output_path         = %q
}`, filepath.Join(dir, "b.tgz")),
	)
}
