package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// API types and calls shared by the user, team and team membership
// resources and data sources.

type portainerUser struct {
	ID       int64  `json:"Id"`
	Username string `json:"Username"`
	Role     int64  `json:"Role"`
	// CreatedAuthenticationMethod / LastAuthenticationMethod were added in
	// Portainer 3.0 (portainer.AuthenticationMethod: 1 = internal, 2 = LDAP,
	// 3 = OAuth). Older servers omit them, which decodes to 0.
	CreatedAuthenticationMethod int `json:"CreatedAuthenticationMethod"`
	LastAuthenticationMethod    int `json:"LastAuthenticationMethod"`
}

// userAuthMethodDescription documents the values of the 3.0 authentication
// method attributes, shared by the resource and the data source.
const userAuthMethodDescription = "One of `internal`, `ldap` or `oauth` (Portainer `portainer.AuthenticationMethod` 1/2/3). Empty on Portainer releases before 3.0, which do not report it, and when the server has not recorded it."

// Descriptions of the two authentication method attributes.
const (
	userCreatedAuthMethodDescription = "Authentication method used when the user was created (Portainer 3.0+). " + userAuthMethodDescription
	userLastAuthMethodDescription    = "Authentication method used at the user's last login (Portainer 3.0+). " + userAuthMethodDescription
)

// authenticationMethodName maps a portainer.AuthenticationMethod value to the
// string exposed in state. 0 (unset, or a pre-3.0 server) maps to "".
func authenticationMethodName(m int) string {
	switch m {
	case 0:
		return ""
	case 1:
		return "internal"
	case 2:
		return "ldap"
	case 3:
		return "oauth"
	default:
		return strconv.Itoa(m)
	}
}

// userAuthMethodValues returns the created_authentication_method and
// last_authentication_method values of a user ("" when not reported).
func userAuthMethodValues(u portainerUser) (created, last types.String) {
	return types.StringValue(authenticationMethodName(u.CreatedAuthenticationMethod)),
		types.StringValue(authenticationMethodName(u.LastAuthenticationMethod))
}

// listUsers returns every user (GET /users).
func listUsers(ctx context.Context, client *APIClient) ([]portainerUser, error) {
	var userList []portainerUser
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/users", nil, &userList); err != nil {
		return nil, err
	}
	return userList, nil
}

type userTeamMembership struct {
	ID     int `json:"Id"`
	UserID int `json:"UserID"`
	TeamID int `json:"TeamID"`
}

// findUserTeamMembership returns the user's membership in the team, or nil.
func findUserTeamMembership(ctx context.Context, client *APIClient, userID, teamID int) (*userTeamMembership, error) {
	var memberships []userTeamMembership
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/team_memberships", nil, &memberships); err != nil {
		return nil, fmt.Errorf("failed to list team memberships: %w", err)
	}
	for i := range memberships {
		if memberships[i].UserID == userID && memberships[i].TeamID == teamID {
			return &memberships[i], nil
		}
	}
	return nil, nil
}

// updateUserTeam moves the user from the old team_id to the new one. A
// membership that already exists (e.g. after an import) is kept.
func updateUserTeam(ctx context.Context, client *APIClient, userID, oldTeam, newTeam int) error {
	if oldTeam != 0 {
		membership, err := findUserTeamMembership(ctx, client, userID, oldTeam)
		if err != nil {
			return err
		}
		if membership != nil {
			url := fmt.Sprintf("%s/team_memberships/%d", client.Endpoint, membership.ID)
			if err := doJSON(ctx, client, http.MethodDelete, url, nil, nil); err != nil && !isAPINotFound(err) {
				return fmt.Errorf("failed to remove user from team %d: %w", oldTeam, err)
			}
		}
	}
	if newTeam == 0 {
		return nil
	}
	membership, err := findUserTeamMembership(ctx, client, userID, newTeam)
	if err != nil || membership != nil {
		return err
	}
	body := map[string]int{"userID": userID, "teamID": newTeam, "role": 2}
	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/team_memberships", body, nil); err != nil {
		return fmt.Errorf("failed to assign user to team %d: %w", newTeam, err)
	}
	return nil
}

type portainerTeam struct {
	ID   int64  `json:"Id"`
	Name string `json:"Name"`
}

// listTeams returns every team (GET /teams).
func listTeams(ctx context.Context, client *APIClient) ([]portainerTeam, error) {
	var teamList []portainerTeam
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/teams", nil, &teamList); err != nil {
		return nil, fmt.Errorf("failed to list teams: %w", err)
	}
	return teamList, nil
}

// teamMembershipPayload is the POST /team_memberships and
// PUT /team_memberships/{id} body.
type teamMembershipPayload struct {
	Role   int64 `json:"role"`
	TeamID int64 `json:"teamID"`
	UserID int64 `json:"userID"`
}

type portainerTeamMembership struct {
	ID     int64 `json:"Id"`
	Role   int64 `json:"Role"`
	TeamID int64 `json:"TeamID"`
	UserID int64 `json:"UserID"`
}

// listTeamMemberships returns every team membership (GET /team_memberships).
func listTeamMemberships(ctx context.Context, client *APIClient) ([]portainerTeamMembership, error) {
	var memberships []portainerTeamMembership
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/team_memberships", nil, &memberships); err != nil {
		return nil, fmt.Errorf("failed to fetch team memberships list: %w", err)
	}
	return memberships, nil
}

// portainerAPIKey is the portainer.APIKey object. The digest is deliberately
// not mapped: it is the stored hash of the key and is of no use in a plan.
type portainerAPIKey struct {
	ID          int    `json:"id"`
	UserID      int    `json:"userId"`
	Description string `json:"description"`
	Prefix      string `json:"prefix"`
	DateCreated int    `json:"dateCreated"`
	LastUsed    int    `json:"lastUsed"`
}

// adminExists asks the public GET /users/admin/check whether the instance has
// an administrator.
func adminExists(ctx context.Context, client *APIClient) (bool, error) {
	if err := doJSONPublic(ctx, client, http.MethodGet, client.Endpoint+"/users/admin/check", nil, nil, nil); err != nil {
		if isAPINotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check for an admin user: %w", err)
	}
	return true, nil
}
