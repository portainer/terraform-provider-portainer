package internal

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// fwIdentityCase is one resource with the { id } identity and its list
// resource, run against the resource's fake API.
type fwIdentityCase struct {
	typ   string // "portainer_tag"
	setup func(m *MockServer)
	// config is the resource block, named "r".
	config string
	// display is the display name of the created object in a query.
	display string
	// values are checked on the listed resource (include_resource).
	values map[string]knownvalue.Check
	// filter is a list configuration that matches the object, miss one
	// that matches nothing ("" when the list has no filter).
	filter, miss string
	// importChanges: the configuration sets values Portainer never returns
	// (passwords), so the plan after an import is not empty.
	importChanges bool
}

var fwNumericID = knownvalue.StringRegexp(regexp.MustCompile(`^[0-9]+$`))

// testFWIdentity: the identity is stored on create and the resource imports
// by identity (an import block with `identity = { id = ... }`).
func testFWIdentity(t *testing.T, c fwIdentityCase) {
	t.Helper()
	mock := NewMockServer(t)
	c.setup(mock)
	addr := c.typ + ".r"
	config := mockProviderConfig(mock) + c.config
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_12_0), skipOnOpenTofuIdentity},
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectIdentity(addr, map[string]knownvalue.Check{"id": fwNumericID}),
					statecheck.ExpectIdentityValueMatchesState(addr, tfjsonpath.New("id")),
				},
			},
			{
				Config:             config,
				ResourceName:       addr,
				ImportState:        true,
				ImportStateKind:    resource.ImportBlockWithResourceIdentity,
				ExpectNonEmptyPlan: c.importChanges,
			},
		},
	})
}

// testFWList: `terraform query` lists the created object with its identity
// and, with include_resource, the state the resource's Read builds; a filter
// narrows the list.
func testFWList(t *testing.T, c fwIdentityCase) {
	t.Helper()
	mock := NewMockServer(t)
	c.setup(mock)
	query := fmt.Sprintf(`
list %q "all" {
  provider         = portainer
  include_resource = true
}
`, c.typ)
	checks := []querycheck.QueryResultCheck{
		querycheck.ExpectLengthAtLeast(c.typ+".all", 1),
		querycheck.ExpectResourceDisplayName(c.typ+".all", queryfilter.ByDisplayName(knownvalue.StringExact(c.display)), knownvalue.StringExact(c.display)),
	}
	values := make([]querycheck.KnownValueCheck, 0, len(c.values))
	for k, v := range c.values {
		values = append(values, querycheck.KnownValueCheck{Path: tfjsonpath.New(k), KnownValue: v})
	}
	checks = append(checks, querycheck.ExpectResourceKnownValues(c.typ+".all", queryfilter.ByDisplayName(knownvalue.StringExact(c.display)), values))
	if c.filter != "" {
		query += fmt.Sprintf(`
list %q "match" {
  provider = portainer
  config {
    %s
  }
}

list %q "miss" {
  provider = portainer
  config {
    %s
  }
}
`, c.typ, c.filter, c.typ, c.miss)
		checks = append(checks,
			querycheck.ExpectLength(c.typ+".match", 1),
			querycheck.ExpectIdentity(c.typ+".match", map[string]knownvalue.Check{"id": fwNumericID}),
			querycheck.ExpectLength(c.typ+".miss", 0),
		)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_14_0), skipOnOpenTofuList},
		Steps: []resource.TestStep{
			{Config: mockProviderConfig(mock) + c.config},
			{Query: true, Config: query, QueryResultChecks: checks},
		},
	})
}

var fwTagCase = fwIdentityCase{
	typ:     "portainer_tag",
	setup:   func(m *MockServer) { f := &fakeTags{}; f.register(m); f.registerDelete(m, "1") },
	config:  `resource "portainer_tag" "r" { name = "production" }`,
	display: "production",
	values:  map[string]knownvalue.Check{"id": knownvalue.StringExact("1"), "name": knownvalue.StringExact("production")},
	filter:  `name = "production"`,
	miss:    `name = "staging"`,
}

func TestTagResource_Identity(t *testing.T) {
	t.Parallel()
	testFWIdentity(t, fwTagCase)
}

func TestTagResource_List(t *testing.T) {
	t.Parallel()
	testFWList(t, fwTagCase)
}

// TestTagResource_IdentityAfterUpgradeFromV300: state written by v3.0.0 has
// no identity. The first refresh stores it, even when the tag is gone and
// Read removes it (the framework rejects a Read that leaves it null).
func TestTagResource_IdentityAfterUpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := &fakeTags{}
	fake.register(mock)
	fake.registerDelete(mock, "1")
	config := mockProviderConfig(mock) + `resource "portainer_tag" "t" { name = "production" }`
	resource.Test(t, resource.TestCase{
		IsUnitTest:             true,
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_12_0), skipOnOpenTofuIdentity},
		Steps: []resource.TestStep{
			{ExternalProviders: previousRelease, Config: config},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   config,
				ConfigPlanChecks:         resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectIdentity("portainer_tag.t", map[string]knownvalue.Check{"id": knownvalue.StringExact("1")}),
				},
			},
		},
	})

	// A v3.0.0 state whose tag was deleted outside Terraform.
	mock2 := NewMockServer(t)
	fake2 := &fakeTags{}
	fake2.register(mock2)
	fake2.registerDelete(mock2, "1")
	fake2.registerDelete(mock2, "2")
	config2 := mockProviderConfig(mock2) + `resource "portainer_tag" "t" { name = "production" }`
	resource.Test(t, resource.TestCase{
		IsUnitTest:             true,
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_12_0), skipOnOpenTofuIdentity},
		Steps: []resource.TestStep{
			{ExternalProviders: previousRelease, Config: config2},
			{
				PreConfig: func() {
					fake2.mu.Lock()
					fake2.tags = nil
					fake2.mu.Unlock()
				},
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   config2,
				PlanOnly:                 true,
				ExpectNonEmptyPlan:       true,
			},
		},
	})
}

var fwTeamCase = fwIdentityCase{
	typ:     "portainer_team",
	setup:   func(m *MockServer) { newFakeUsersAPI(m) },
	config:  `resource "portainer_team" "r" { name = "devops" }`,
	display: "devops",
	values:  map[string]knownvalue.Check{"id": fwNumericID, "name": knownvalue.StringExact("devops")},
	filter:  `name = "devops"`,
	miss:    `name = "platform"`,
}

func TestTeamResource_Identity(t *testing.T) {
	t.Parallel()
	testFWIdentity(t, fwTeamCase)
}

func TestTeamResource_List(t *testing.T) {
	t.Parallel()
	testFWList(t, fwTeamCase)
}

var fwUserCase = fwIdentityCase{
	typ:   "portainer_user",
	setup: func(m *MockServer) { newFakeUsersAPI(m) },
	config: `resource "portainer_user" "r" {
  username = "alice"
  password = "pw-1234567890"
  role     = 2
}`,
	display: "alice",
	values: map[string]knownvalue.Check{
		"username": knownvalue.StringExact("alice"),
		"role":     knownvalue.Int64Exact(2),
		"password": knownvalue.Null(),
	},
	filter: `username = "alice"
    role     = 2`,
	miss:          `username = "bob"`,
	importChanges: true, // the password is never read back
}

func TestUserResource_Identity(t *testing.T) {
	t.Parallel()
	testFWIdentity(t, fwUserCase)
}

func TestUserResource_List(t *testing.T) {
	t.Parallel()
	testFWList(t, fwUserCase)
}

var fwEndpointGroupCase = fwIdentityCase{
	typ:   "portainer_endpoint_group",
	setup: func(m *MockServer) { newFakeEndpointGroups(m) },
	config: `resource "portainer_endpoint_group" "r" {
  name        = "prod"
  description = "d"
}`,
	display: "prod",
	values:  map[string]knownvalue.Check{"name": knownvalue.StringExact("prod"), "description": knownvalue.StringExact("d")},
	filter:  `name = "prod"`,
	miss:    `name = "dev"`,
}

func TestEndpointGroupResource_Identity(t *testing.T) {
	t.Parallel()
	testFWIdentity(t, fwEndpointGroupCase)
}

func TestEndpointGroupResource_List(t *testing.T) {
	t.Parallel()
	testFWList(t, fwEndpointGroupCase)
}

var fwEdgeGroupCase = fwIdentityCase{
	typ:     "portainer_edge_group",
	setup:   func(m *MockServer) { newFakeEdgeGroups(m, 1) },
	config:  strings.Replace(edgeGroupHCL("edge"), `"g"`, `"r"`, 1),
	display: "edge",
	values:  map[string]knownvalue.Check{"id": knownvalue.StringExact("1"), "name": knownvalue.StringExact("edge"), "dynamic": knownvalue.Bool(true)},
	filter:  `name = "edge"`,
	miss:    `name = "other"`,
}

func TestEdgeGroupResource_Identity(t *testing.T) {
	t.Parallel()
	testFWIdentity(t, fwEdgeGroupCase)
}

func TestEdgeGroupResource_List(t *testing.T) {
	t.Parallel()
	testFWList(t, fwEdgeGroupCase)
}

var fwEnvironmentCase = fwIdentityCase{
	typ:   "portainer_environment",
	setup: func(m *MockServer) { newFakeEnvironments(m) },
	config: `resource "portainer_environment" "r" {
  name                = "prod"
  environment_address = "tcp://docker.example:2375"
  type                = 1
}`,
	display: "prod",
	values: map[string]knownvalue.Check{
		"name":                knownvalue.StringExact("prod"),
		"type":                knownvalue.Int64Exact(1),
		"environment_address": knownvalue.StringExact("tcp://docker.example:2375"),
	},
	filter: `name     = "prod"
    group_id = 1`,
	miss: `group_id = 99`,
}

func TestEnvironmentResource_Identity(t *testing.T) {
	t.Parallel()
	testFWIdentity(t, fwEnvironmentCase)
}

func TestEnvironmentResource_List(t *testing.T) {
	t.Parallel()
	testFWList(t, fwEnvironmentCase)
}

var fwRegistryCase = fwIdentityCase{
	typ:   "portainer_registry",
	setup: func(m *MockServer) { newFakeRegistries(m) },
	config: `resource "portainer_registry" "r" {
  name = "harbor"
  url  = "registry.example.com"
  type = 3
}`,
	display: "harbor",
	values:  map[string]knownvalue.Check{"name": knownvalue.StringExact("harbor"), "url": knownvalue.StringExact("registry.example.com"), "type": knownvalue.Int64Exact(3)},
	filter: `name = "harbor"
    type = 3`,
	miss: `type = 6`,
}

func TestRegistryResource_Identity(t *testing.T) {
	t.Parallel()
	testFWIdentity(t, fwRegistryCase)
}

func TestRegistryResource_List(t *testing.T) {
	t.Parallel()
	testFWList(t, fwRegistryCase)
}

var fwCustomTemplateCase = fwIdentityCase{
	typ:     "portainer_custom_template",
	setup:   func(m *MockServer) { newFakeCustomTemplates(m) },
	config:  strings.Replace(customTemplateConfig(`file_content = "services: {}"`), `"t"`, `"r"`, 1),
	display: "web",
	values:  map[string]knownvalue.Check{"title": knownvalue.StringExact("web"), "description": knownvalue.StringExact("Web stack"), "type": knownvalue.Int64Exact(2)},
	filter:  `title = "web"`,
	miss:    `title = "db"`,
	// file_content is not read back, as with an import by ID.
	importChanges: true,
}

func TestCustomTemplateResource_Identity(t *testing.T) {
	t.Parallel()
	testFWIdentity(t, fwCustomTemplateCase)
}

func TestCustomTemplateResource_List(t *testing.T) {
	t.Parallel()
	testFWList(t, fwCustomTemplateCase)
}

var fwWebhookCase = fwIdentityCase{
	typ:   "portainer_webhook",
	setup: func(m *MockServer) { newFakeWebhooks(m) },
	config: `resource "portainer_webhook" "r" {
  endpoint_id  = 1
  resource_id  = "svc-1"
  webhook_type = 1
}`,
	display: "webhook 1 (svc-1)",
	values: map[string]knownvalue.Check{
		"endpoint_id":  knownvalue.Int64Exact(1),
		"resource_id":  knownvalue.StringExact("svc-1"),
		"webhook_type": knownvalue.Int64Exact(1),
		"token":        knownvalue.StringExact("tok-1"),
	},
	filter: `endpoint_id = 1
    resource_id = "svc-1"`,
	miss: `endpoint_id = 2`,
}

func TestWebhookResource_Identity(t *testing.T) {
	t.Parallel()
	testFWIdentity(t, fwWebhookCase)
}

func TestWebhookResource_List(t *testing.T) {
	t.Parallel()
	testFWList(t, fwWebhookCase)
}

// TestAccTagResource_ListAndIdentity runs against a real Portainer (CE is
// enough): the created tag is found by `terraform query` and imports by
// identity.
func TestAccTagResource_ListAndIdentity(t *testing.T) {
	name := acctest.RandomWithPrefix("acc-tag")
	config := accProvider() + fmt.Sprintf(`resource "portainer_tag" "r" { name = %q }`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV5ProviderFactories: testProtoV5Factories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_14_0), skipOnOpenTofuIdentity, skipOnOpenTofuList},
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectIdentityValueMatchesState("portainer_tag.r", tfjsonpath.New("id")),
				},
			},
			{
				Config:          config,
				ResourceName:    "portainer_tag.r",
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithResourceIdentity,
			},
			{
				Query: true,
				Config: fmt.Sprintf(`
list "portainer_tag" "match" {
  provider         = portainer
  include_resource = true
  config {
    name = %q
  }
}
`, name),
				QueryResultChecks: []querycheck.QueryResultCheck{
					querycheck.ExpectLength("portainer_tag.match", 1),
					querycheck.ExpectResourceKnownValues("portainer_tag.match", queryfilter.ByDisplayName(knownvalue.StringExact(name)), []querycheck.KnownValueCheck{
						{Path: tfjsonpath.New("name"), KnownValue: knownvalue.StringExact(name)},
					}),
				},
			},
		},
	})
}
