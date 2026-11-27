package internal

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Portainer Business Edition 3.1 is an OIDC provider: other applications and
// add-ons sign users in through it. An administrator registers each of them
// as an OIDC client (/oidc_clients), which holds client secrets and the
// sessions of the users signed in to it.

// oidcAccessPolicy is portainer.AccessPolicy. An OIDC client only looks at
// whether a user or team has a policy; Portainer still requires a role ID.
type oidcAccessPolicy struct {
	RoleID int64 `json:"RoleId"`
}

// oidcSecretAPI is oidcclient.Secret. Its digest is left out: it is a hash
// of the value, of no use to a configuration.
type oidcSecretAPI struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Hint        string `json:"hint"`
	CreatedAt   int64  `json:"createdAt"`
	ExpiresAt   int64  `json:"expiresAt"`
}

// oidcClientAPI is oidcclient.Client, as GET /oidc_clients/{id} returns it.
type oidcClientAPI struct {
	ID                     int64                       `json:"id"`
	ClientID               string                      `json:"clientId"`
	Name                   string                      `json:"name"`
	Description            string                      `json:"description"`
	HomepageURL            string                      `json:"homepageUrl"`
	Permission             string                      `json:"permission"`
	AllUsers               bool                        `json:"allUsers"`
	Enabled                bool                        `json:"enabled"`
	GroupsClaim            bool                        `json:"groupsClaim"`
	ShowInLauncher         bool                        `json:"showInLauncher"`
	RedirectURIs           []string                    `json:"redirectUris"`
	PostLogoutRedirectURIs []string                    `json:"postLogoutRedirectUris"`
	Secrets                []oidcSecretAPI             `json:"secrets"`
	UserAccessPolicies     map[string]oidcAccessPolicy `json:"userAccessPolicies"`
	TeamAccessPolicies     map[string]oidcAccessPolicy `json:"teamAccessPolicies"`
}

// oidcSecretCreateResponse is oidcclients.secretCreateResponse: the secret
// and its value, which Portainer returns only here.
type oidcSecretCreateResponse struct {
	Secret oidcSecretAPI `json:"secret"`
	Value  string        `json:"value"`
}

// oidcGetClient reads one client (GET /oidc_clients/{id}).
func oidcGetClient(ctx context.Context, client *APIClient, id int64) (oidcClientAPI, error) {
	var c oidcClientAPI
	err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/oidc_clients/%d", client.Endpoint, id), nil, &c)
	return c, err
}

// oidcListClients lists the clients (GET /oidc_clients), ordered by ID.
func oidcListClients(ctx context.Context, client *APIClient, feature string) ([]oidcClientAPI, error) {
	var clients []oidcClientAPI
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/oidc_clients", nil, &clients); err != nil {
		err = explainUnsupported(ctx, client, err, feature, requiresBE31OIDC)
		return nil, fmt.Errorf("failed to list OIDC clients: %w", err)
	}
	sort.Slice(clients, func(i, j int) bool { return clients[i].ID < clients[j].ID })
	return clients, nil
}

// oidcPoliciesFromMap turns a map(number) of user or team IDs to role IDs
// into the access policies Portainer stores. The result is never nil, so an
// update that drops every policy sends an empty object.
func oidcPoliciesFromMap(ctx context.Context, m types.Map, diags *diag.Diagnostics) map[string]oidcAccessPolicy {
	out := map[string]oidcAccessPolicy{}
	if m.IsNull() || m.IsUnknown() {
		return out
	}
	var roles map[string]int64
	diags.Append(m.ElementsAs(ctx, &roles, false)...)
	for id, role := range roles {
		out[id] = oidcAccessPolicy{RoleID: role}
	}
	return out
}

// oidcPoliciesValue is the inverse: null when Portainer has no policy and
// the prior value was null.
func oidcPoliciesValue(api map[string]oidcAccessPolicy, prior types.Map) types.Map {
	if len(api) == 0 && prior.IsNull() {
		return types.MapNull(types.Int64Type)
	}
	elems := make(map[string]attr.Value, len(api))
	for id, p := range api {
		elems[id] = types.Int64Value(p.RoleID)
	}
	return types.MapValueMust(types.Int64Type, elems)
}

// oidcOptStringList is the state value of an optional list(string): null
// when Portainer has no element and the prior value was null.
func oidcOptStringList(api []string, prior types.List) types.List {
	if len(api) == 0 && prior.IsNull() {
		return types.ListNull(types.StringType)
	}
	return oidcStringList(api)
}

func oidcStringList(api []string) types.List {
	elems := make([]attr.Value, 0, len(api))
	for _, s := range api {
		elems = append(elems, types.StringValue(s))
	}
	return types.ListValueMust(types.StringType, elems)
}

// oidcStrings reads a list(string) ([] for null or unknown).
func oidcStrings(ctx context.Context, l types.List, diags *diag.Diagnostics) []string {
	out := []string{}
	if l.IsNull() || l.IsUnknown() {
		return out
	}
	diags.Append(l.ElementsAs(ctx, &out, false)...)
	return out
}

// oidcParseID parses a numeric Portainer ID stored as a string.
func oidcParseID(s string) (int64, error) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid OIDC client ID %q: %w", s, err)
	}
	return id, nil
}

// oidcSecretAttrTypes is an element of the `secrets` of a client in
// portainer_oidc_clients.
var oidcSecretAttrTypes = map[string]attr.Type{
	"id":          types.StringType,
	"description": types.StringType,
	"hint":        types.StringType,
	"created_at":  types.Int64Type,
	"expires_at":  types.Int64Type,
}

func oidcSecretsValue(secrets []oidcSecretAPI) types.List {
	elems := make([]attr.Value, 0, len(secrets))
	for _, s := range secrets {
		elems = append(elems, types.ObjectValueMust(oidcSecretAttrTypes, map[string]attr.Value{
			"id":          types.StringValue(s.ID),
			"description": types.StringValue(s.Description),
			"hint":        types.StringValue(s.Hint),
			"created_at":  types.Int64Value(s.CreatedAt),
			"expires_at":  types.Int64Value(s.ExpiresAt),
		}))
	}
	return types.ListValueMust(types.ObjectType{AttrTypes: oidcSecretAttrTypes}, elems)
}
