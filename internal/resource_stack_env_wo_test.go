package internal

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// env_wo values are write-only: they never reach state, so the handlers read
// them from the raw configuration. These tests hand the raw configuration in
// through the prior state, which GetRawConfig falls back to.

// stackDataWithWriteOnlyEnv builds a stack ResourceData whose state holds
// TZ=UTC in env and the name DB_PASSWORD in env_wo, and whose raw config
// carries the DB_PASSWORD value.
func stackDataWithWriteOnlyEnv(t *testing.T, id string) (*schema.Resource, *schema.ResourceData) {
	t.Helper()
	r := resourcePortainerStack()
	d := r.Data(&terraform.InstanceState{
		ID: id,
		Attributes: map[string]string{
			"id":                        id,
			"env.#":                     "1",
			"env.0.name":                "TZ",
			"env.0.value":               "UTC",
			"env_wo.#":                  "1",
			"env_wo.0.name":             "DB_PASSWORD",
			"env_wo_version":            "1",
			"deployment_type":           "standalone",
			"method":                    "string",
			"name":                      "web",
			"endpoint_id":               "1",
			"active":                    "true",
			"repository_reference_name": "refs/heads/main",
		},
		RawConfig: cty.ObjectVal(map[string]cty.Value{
			"env_wo": cty.ListVal([]cty.Value{
				cty.ObjectVal(map[string]cty.Value{
					"name":  cty.StringVal("DB_PASSWORD"),
					"value": cty.StringVal("s3cret"),
				}),
			}),
		}),
	})
	_ = d.Set("stack_file_content", "version: '3'")
	return r, d
}

func envPayloadMap(t *testing.T, req *RecordedRequest) map[string]string {
	t.Helper()
	var payload struct {
		Env []map[string]string `json:"env"`
	}
	if err := req.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	out := map[string]string{}
	for _, e := range payload.Env {
		out[e["name"]] = e["value"]
	}
	return out
}

func TestStackEnvPayload_AppendsWriteOnlyEnv(t *testing.T) {
	_, d := stackDataWithWriteOnlyEnv(t, "5")

	got := stackEnvPayload(d)
	if len(got) != 2 {
		t.Fatalf("expected 2 variables, got %v", got)
	}
	if got[0]["name"] != "TZ" || got[0]["value"] != "UTC" {
		t.Errorf("env[0]: expected TZ=UTC, got %v", got[0])
	}
	if got[1]["name"] != "DB_PASSWORD" || got[1]["value"] != "s3cret" {
		t.Errorf("env[1]: expected DB_PASSWORD=s3cret, got %v", got[1])
	}
}

func TestStackEnvPayload_WithoutWriteOnlyEnv(t *testing.T) {
	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("env", []interface{}{map[string]interface{}{"name": "TZ", "value": "UTC"}})

	got := stackEnvPayload(d)
	if len(got) != 1 || got[0]["name"] != "TZ" {
		t.Errorf("expected only TZ, got %v", got)
	}
}

// TestStackCreate_WriteOnlyEnv covers the whole Create: both the create POST
// and the finalize PUT carry the secret, and Read keeps it out of env.
func TestStackCreate_WriteOnlyEnv(t *testing.T) {
	mock := NewMockServer(t)
	mockEmptyStackList(mock)
	mock.On("POST", "/stacks/create/standalone/string", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 5, "Name": "web"}))
	mock.On("PUT", "/stacks/5", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 5, "Name": "web"}))
	mock.On("GET", "/stacks/5", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 5, "Name": "web", "Status": 1, "Type": 2, "EndpointId": 1,
		"Env": []map[string]interface{}{
			{"name": "TZ", "value": "UTC"},
			{"name": "DB_PASSWORD", "value": "s3cret"},
		},
	}))
	mock.On("GET", "/stacks/5/file", RespondJSON(http.StatusOK, map[string]interface{}{"StackFileContent": "version: '3'"}))

	r, d := stackDataWithWriteOnlyEnv(t, "")
	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	for _, req := range []*RecordedRequest{
		mock.FindRequest("POST", "/stacks/create/standalone/string"),
		mock.FindRequest("PUT", "/stacks/5"),
	} {
		if req == nil {
			t.Fatal("expected create POST and finalize PUT")
		}
		env := envPayloadMap(t, req)
		if env["TZ"] != "UTC" || env["DB_PASSWORD"] != "s3cret" {
			t.Errorf("%s %s: expected TZ and DB_PASSWORD in env, got %v", req.Method, req.Path, env)
		}
	}

	env := d.Get("env").([]interface{})
	if len(env) != 1 || env[0].(map[string]interface{})["name"] != "TZ" {
		t.Errorf("expected only TZ in env state, got %v", env)
	}
	if strings.Contains(stateString(d), "s3cret") {
		t.Error("the env_wo value leaked into state")
	}
}

// TestStackUpdate_WriteOnlyEnvSentWithoutVersionChange guards the reason the
// secret is sent on every update: Portainer replaces the whole env list, so an
// update for another attribute would otherwise drop it from the stack.
func TestStackUpdate_WriteOnlyEnvSentWithoutVersionChange(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/stacks/5", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 5, "Name": "web"}))
	mock.On("GET", "/stacks/5", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 5, "Name": "web", "Status": 1, "Type": 2, "EndpointId": 1,
	}))
	mock.On("GET", "/stacks/5/file", RespondJSON(http.StatusOK, map[string]interface{}{"StackFileContent": "version: '3.8'"}))

	r, d := stackDataWithWriteOnlyEnv(t, "5")
	_ = d.Set("stack_file_content", "version: '3.8'")
	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/stacks/5")
	if put == nil {
		t.Fatal("expected PUT /stacks/5")
	}
	if env := envPayloadMap(t, put); env["DB_PASSWORD"] != "s3cret" {
		t.Errorf("expected DB_PASSWORD in the update, got %v", env)
	}
}

func TestStackRead_DropsWriteOnlyEnv(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/stacks/7", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 7, "Name": "web", "Status": 1, "Type": 2, "EndpointId": 1,
		"Env": []map[string]interface{}{
			{"name": "TZ", "value": "UTC"},
			{"name": "DB_PASSWORD", "value": "s3cret"},
		},
	}))
	mock.On("GET", "/stacks/7/file", RespondJSON(http.StatusOK, map[string]interface{}{"StackFileContent": "version: '3'"}))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	d.SetId("7")
	_ = d.Set("method", "string")
	_ = d.Set("env_wo", []interface{}{map[string]interface{}{"name": "DB_PASSWORD"}})

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	env := d.Get("env").([]interface{})
	if len(env) != 1 || env[0].(map[string]interface{})["name"] != "TZ" {
		t.Errorf("expected only TZ in env, got %v", env)
	}
}

func TestStackDiff_WriteOnlyEnvValidation(t *testing.T) {
	base := func() map[string]interface{} {
		return map[string]interface{}{
			"deployment_type":    "standalone",
			"method":             "string",
			"name":               "web",
			"endpoint_id":        1,
			"stack_file_content": "version: '3'",
		}
	}
	envItem := func(name string) map[string]interface{} {
		return map[string]interface{}{"name": name, "value": "x"}
	}

	cases := map[string]struct {
		env, envWO []interface{}
		wantErr    string
	}{
		"distinct names": {
			env:   []interface{}{envItem("TZ")},
			envWO: []interface{}{envItem("DB_PASSWORD")},
		},
		"name in env and env_wo": {
			env:     []interface{}{envItem("DB_PASSWORD")},
			envWO:   []interface{}{envItem("DB_PASSWORD")},
			wantErr: "declared in both env and env_wo",
		},
		"name twice in env_wo": {
			envWO:   []interface{}{envItem("DB_PASSWORD"), envItem("DB_PASSWORD")},
			wantErr: "declared more than once",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			raw := base()
			if tc.env != nil {
				raw["env"] = tc.env
			}
			if tc.envWO != nil {
				raw["env_wo"] = tc.envWO
			}
			_, err := resourcePortainerStack().Diff(context.Background(), nil, terraform.NewResourceConfigRaw(raw), nil)
			switch {
			case tc.wantErr == "" && err != nil:
				t.Errorf("unexpected error: %v", err)
			case tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)):
				t.Errorf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func stateString(d *schema.ResourceData) string {
	var b strings.Builder
	for k, v := range d.State().Attributes {
		b.WriteString(k + "=" + v + "\n")
	}
	return b.String()
}

// TestStackUpdate_Repository_WriteOnlyEnv covers the git path: both the git
// settings update and the redeploy replace the stack's env, so both must carry
// the env_wo values.
func TestStackUpdate_Repository_WriteOnlyEnv(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/stacks/5/git", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("PUT", "/stacks/5/git/redeploy", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/stacks/5", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 5, "Name": "web", "Status": 1, "Type": 2, "EndpointId": 1,
		"Env": []map[string]interface{}{
			{"name": "TZ", "value": "UTC"},
			{"name": "DB_PASSWORD", "value": "s3cret"},
		},
		"gitConfig": map[string]interface{}{
			"URL":           "https://github.com/acme/app.git",
			"ReferenceName": "refs/heads/main",
		},
	}))

	r, d := stackDataWithWriteOnlyEnv(t, "5")
	_ = d.Set("method", "repository")
	_ = d.Set("repository_url", "https://github.com/acme/app.git")
	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	for _, req := range []*RecordedRequest{
		mock.FindRequest("POST", "/stacks/5/git"),
		mock.FindRequest("PUT", "/stacks/5/git/redeploy"),
	} {
		if req == nil {
			t.Fatal("expected the git update and the redeploy")
		}
		env := envPayloadMap(t, req)
		if env["TZ"] != "UTC" || env["DB_PASSWORD"] != "s3cret" {
			t.Errorf("%s %s: expected TZ and DB_PASSWORD in env, got %v", req.Method, req.Path, env)
		}
	}
	env := d.Get("env").([]interface{})
	if len(env) != 1 || env[0].(map[string]interface{})["name"] != "TZ" {
		t.Errorf("expected only TZ in env state, got %v", env)
	}
}
