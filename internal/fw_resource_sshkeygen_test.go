package internal

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const (
	sshkeygenTestPublic  = "ssh-rsa AAAAB3NzaC1yc2EAAAAD test@host"
	sshkeygenTestPrivate = "-----BEGIN RSA PRIVATE KEY-----\nfake\n-----END RSA PRIVATE KEY-----"
)

func sshkeygenMock(t *testing.T) *MockServer {
	mock := NewMockServer(t)
	mock.On("POST", "/sshkeygen", RespondJSON(http.StatusOK, map[string]string{
		"public": sshkeygenTestPublic, "private": sshkeygenTestPrivate,
	}))
	return mock
}

// TestSSHKeygenResource_Create: the keys are stored, the ID is the length of
// the public key and the request carries the API key.
func TestSSHKeygenResource_Create(t *testing.T) {
	t.Parallel()
	mock := sshkeygenMock(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `resource "portainer_sshkeygen" "k" {}`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("portainer_sshkeygen.k", "id", strconv.Itoa(len(sshkeygenTestPublic))),
				resource.TestCheckResourceAttr("portainer_sshkeygen.k", "public", sshkeygenTestPublic),
				resource.TestCheckResourceAttr("portainer_sshkeygen.k", "private", sshkeygenTestPrivate),
				func(*terraform.State) error {
					req := mock.FindRequest("POST", "/sshkeygen")
					if req == nil || req.Headers.Get("X-API-Key") == "" {
						return fmt.Errorf("expected POST /sshkeygen with the X-API-Key header")
					}
					return nil
				},
			),
		}},
	})
}

func TestSSHKeygenResource_HTTPError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/sshkeygen", RespondString(http.StatusInternalServerError, "application/json", `{"message":"keygen failed"}`))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `resource "portainer_sshkeygen" "k" {}`,
			ExpectError: regexp.MustCompile(`failed to generate SSH key: .*keygen failed`),
		}},
	})
}

func TestSSHKeygenResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := sshkeygenMock(t)
	config := mockProviderConfig(mock) + `resource "portainer_sshkeygen" "k" {}`
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

// TestAccSSHKeygenResource generates a key pair on a real Portainer.
func TestAccSSHKeygenResource(t *testing.T) {
	accRequireBE(t, "SSH key generation")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: accProvider() + `resource "portainer_sshkeygen" "k" {}`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestMatchResourceAttr("portainer_sshkeygen.k", "public", regexp.MustCompile(`^ssh-`)),
				resource.TestCheckResourceAttrSet("portainer_sshkeygen.k", "private"),
			),
		}},
	})
}
