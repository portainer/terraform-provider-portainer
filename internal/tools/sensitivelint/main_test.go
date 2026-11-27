package main

import (
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/portainer/terraform-provider-portainer/internal"
)

func TestLooksSecret(t *testing.T) {
	for name, want := range map[string]bool{
		"password":                 true,
		"api_key":                  true,
		"password_wo":              true,
		"webhook_url":              true,
		"edge_key":                 true,
		"password_wo_version":      false,
		"required_password_length": false,
		"token_name":               false,
		"secret_id":                false,
		"public_key":               false,
		"generate_api_key":         false,
		"name":                     false,
	} {
		if got := looksSecret(name); got != want {
			t.Errorf("looksSecret(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestCheck(t *testing.T) {
	schemas := []internal.LintSchema{{
		Kind:     "data source",
		TypeName: "portainer_x",
		Block: &tfprotov5.SchemaBlock{
			Attributes: []*tfprotov5.SchemaAttribute{
				{Name: "password", Sensitive: true},
				{Name: "api_key"},
				{Name: "password_wo", WriteOnly: true},
				{Name: "name"},
			},
			BlockTypes: []*tfprotov5.SchemaNestedBlock{
				{TypeName: "credentials", Block: &tfprotov5.SchemaBlock{
					Attributes: []*tfprotov5.SchemaAttribute{{Name: "token"}, {Name: "client_secret", Sensitive: true}},
				}},
			},
		},
	}}
	got := check(schemas)
	want := []string{
		`data source portainer_x: secret-like field "api_key" is not marked Sensitive: true`,
		`data source portainer_x: secret-like field "credentials.token" is not marked Sensitive: true`,
		`data source portainer_x: secret-like field "password_wo" is not marked Sensitive: true`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("check() =\n%q\nwant\n%q", got, want)
	}
}
