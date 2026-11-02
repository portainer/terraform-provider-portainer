package internal

import (
	"net/http"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// password_wo never reaches state, so it is read from the raw configuration,
// handed in here through the prior state the way GetRawConfig falls back to.
func registryWithWriteOnlyPassword(id string) *terraform.InstanceState {
	return &terraform.InstanceState{
		ID: id,
		Attributes: map[string]string{
			"id":                  id,
			"name":                "ghcr",
			"url":                 "ghcr.io",
			"type":                "3",
			"authentication":      "true",
			"username":            "bot",
			"password_wo_version": "1",
		},
		RawConfig: cty.ObjectVal(map[string]cty.Value{
			"password_wo": cty.StringVal("s3cret-token"),
		}),
	}
}

func TestRegistryCreate_SendsWriteOnlyPassword(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/registries", RespondJSON(http.StatusOK, []interface{}{}))
	mock.On("POST", "/registries", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 4}))
	mock.On("GET", "/registries/4", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 4, "Name": "ghcr", "URL": "ghcr.io", "Type": 3, "Authentication": true, "Username": "bot"}))

	r := resourceRegistry()
	d := r.Data(registryWithWriteOnlyPassword(""))
	d.SetId("")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	var payload map[string]interface{}
	if err := mock.FindRequest("POST", "/registries").DecodeJSON(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload["password"] != "s3cret-token" {
		t.Errorf("payload.password: expected the write-only value, got %v", payload["password"])
	}
	if got := d.Get("password").(string); got != "" {
		t.Errorf("password must stay out of state, got %q", got)
	}
}

func TestRegistryPassword_WriteOnlyOnlyWhenAsked(t *testing.T) {
	d := resourceRegistry().Data(registryWithWriteOnlyPassword("4"))
	if got, err := registryPassword(d, false); err != nil || got != "" {
		t.Errorf("without a version change no password may be sent (Portainer keeps the stored one), got %q (%v)", got, err)
	}
	if got, err := registryPassword(d, true); err != nil || got != "s3cret-token" {
		t.Errorf("with a version change the write-only password must be sent, got %q (%v)", got, err)
	}
}
