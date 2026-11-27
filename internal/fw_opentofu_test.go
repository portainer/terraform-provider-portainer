package internal

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// The tests run with Terraform or OpenTofu: TF_ACC_TERRAFORM_PATH selects the
// CLI. terraform-plugin-testing does not tell them apart, and OpenTofu reports
// its own version (1.12 passes SkipBelow(1.14)) although it supports neither
// resource identity, list resources nor actions.

// cliIsOpenTofu reports whether the CLI the tests run is OpenTofu: the first
// line of `<cli> version` is "OpenTofu vX.Y.Z" (the -json output has no field
// naming the product). PORTAINER_ACC_OPENTOFU=1 forces it.
var cliIsOpenTofu = sync.OnceValue(func() bool {
	if os.Getenv("PORTAINER_ACC_OPENTOFU") == "1" {
		return true
	}
	path := os.Getenv("TF_ACC_TERRAFORM_PATH")
	if path == "" {
		if os.Getenv("TF_ACC_TERRAFORM_VERSION") != "" {
			return false // terraform-plugin-testing installs Terraform
		}
		var err error
		if path, err = exec.LookPath("terraform"); err != nil {
			return false
		}
	}
	cmd := exec.Command(path, "version")
	cmd.Env = append(os.Environ(), "CHECKPOINT_DISABLE=1")
	out, err := cmd.Output()
	return err == nil && strings.HasPrefix(strings.TrimSpace(string(out)), "OpenTofu")
})

// skipOnOpenTofu skips a test case of a feature OpenTofu lacks (resource
// identity, list resources, actions). Use it next to the SkipBelow check.
type skipOnOpenTofu struct{ feature string }

func (s skipOnOpenTofu) CheckTerraformVersion(_ context.Context, _ tfversion.CheckTerraformVersionRequest, resp *tfversion.CheckTerraformVersionResponse) {
	if cliIsOpenTofu() {
		resp.Skip = "OpenTofu does not support " + s.feature
	}
}

var (
	skipOnOpenTofuIdentity = skipOnOpenTofu{"resource identity"}
	skipOnOpenTofuList     = skipOnOpenTofu{"list resources"}
	skipOnOpenTofuActions  = skipOnOpenTofu{"actions"}
)

// terraform-plugin-testing registers the provider as
// registry.terraform.io/{-,hashicorp}/portainer. OpenTofu rejects the legacy
// "-" namespace on that host (every test would fail before it starts) and
// resolves the unqualified "portainer" of steps that do not reattach the
// provider (import steps, destroy after them) on registry.opentofu.org.
func init() {
	if !cliIsOpenTofu() {
		return
	}
	if os.Getenv("TF_ACC_PROVIDER_NAMESPACE") == "" {
		_ = os.Setenv("TF_ACC_PROVIDER_NAMESPACE", "hashicorp")
	}
	if os.Getenv("TF_ACC_PROVIDER_HOST") == "" {
		_ = os.Setenv("TF_ACC_PROVIDER_HOST", "registry.opentofu.org")
	}
}
