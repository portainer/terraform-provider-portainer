package internal

import (
	"fmt"
	"strconv"

	httptransport "github.com/go-openapi/runtime/client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/portainer/client-api-go/v2/pkg/client/auth"
	"github.com/portainer/client-api-go/v2/pkg/client/team_memberships"
	"github.com/portainer/client-api-go/v2/pkg/client/users"
	"github.com/portainer/client-api-go/v2/pkg/models"
)

func resourceUser() *schema.Resource {
	return &schema.Resource{
		Create: resourceUserCreate,
		Read:   resourceUserRead,
		Delete: resourceUserDelete,
		Update: resourceUserUpdate,

		Importer: &schema.ResourceImporter{
			State: resourceUserImport,
		},

		Schema: map[string]*schema.Schema{
			"username": {
				Type:     schema.TypeString,
				Required: true,
			},
			"password": {
				Type:      schema.TypeString,
				Optional:  true,
				Sensitive: true,
			},
			"role": {
				Type:     schema.TypeInt,
				Optional: true,
				Default:  2,
			},
			"ldap_user": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
				ForceNew: true,
			},
			"team_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Optional Portainer team ID. Only applicable for standard users (role = 2).",
			},
			"generate_api_key": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"api_key_description": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "terraform-generated-api-key",
			},
			"api_key_raw": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourceUserCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	username := d.Get("username").(string)
	password := d.Get("password").(string)
	role := int64(d.Get("role").(int))
	ldapUser := d.Get("ldap_user").(bool)

	if ldapUser && password != "" {
		return fmt.Errorf("cannot set password for LDAP user")
	}
	if !ldapUser && password == "" {
		return fmt.Errorf("password is required for non-LDAP user")
	}

	// Check if user already exists
	paramsList := users.NewUserListParams()
	respList, err := client.Client.Users.UserList(paramsList, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to list users: %w", err)
	}

	for _, u := range respList.Payload {
		if u.Username == username {
			d.SetId(strconv.FormatInt(u.ID, 10))
			return resourceUserUpdate(d, meta)
		}
	}

	paramsCreate := users.NewUserCreateParams()
	paramsCreate.Body = &models.UsersUserCreatePayload{
		Username: &username,
		Role:     &role,
	}
	if !ldapUser {
		paramsCreate.Body.Password = &password
	}

	respCreate, err := client.Client.Users.UserCreate(paramsCreate, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	d.SetId(strconv.FormatInt(respCreate.Payload.ID, 10))

	if teamIDInt, ok := d.GetOk("team_id"); ok {
		if role != 2 {
			return fmt.Errorf("team_id can only be used with standard users (role = 2)")
		}
		teamID := int64(teamIDInt.(int))
		userID := respCreate.Payload.ID
		roleMember := int64(2)

		paramsTeam := team_memberships.NewTeamMembershipCreateParams()
		paramsTeam.Body = &models.TeammembershipsTeamMembershipCreatePayload{
			UserID: &userID,
			TeamID: &teamID,
			Role:   &roleMember,
		}

		_, err := client.Client.TeamMemberships.TeamMembershipCreate(paramsTeam, client.AuthInfo)
		if err != nil {
			return fmt.Errorf("failed to assign user to team: %w", err)
		}
	}

	if d.Get("generate_api_key").(bool) {
		description := d.Get("api_key_description").(string)
		if password == "" {
			return fmt.Errorf("password must be set to generate API key")
		}

		// Authenticate as new user
		paramsAuth := auth.NewAuthenticateUserParams()
		paramsAuth.Body = &models.AuthAuthenticatePayload{
			Username: &username,
			Password: &password,
		}

		respAuth, err := client.Client.Auth.AuthenticateUser(paramsAuth)
		if err != nil {
			return fmt.Errorf("failed to authenticate as new user: %w", err)
		}

		userAuthInfo := httptransport.BearerToken(respAuth.Payload.Jwt)

		paramsKey := users.NewUserGenerateAPIKeyParams()
		paramsKey.ID = respCreate.Payload.ID
		paramsKey.Body = &models.UsersUserAccessTokenCreatePayload{
			Description: &description,
			Password:    &password,
		}

		respKey, err := client.Client.Users.UserGenerateAPIKey(paramsKey, userAuthInfo)
		if err != nil {
			return fmt.Errorf("failed to generate API key: %w", err)
		}

		d.Set("api_key_raw", respKey.Payload.RawAPIKey)
	}

	return resourceUserRead(d, meta)
}

func resourceUserRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	params := users.NewUserInspectParams()
	params.ID = id

	resp, err := client.Client.Users.UserInspect(params, client.AuthInfo)
	if err != nil {
		if _, ok := err.(*users.UserInspectNotFound); ok {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("failed to read user: %w", err)
	}

	d.Set("username", resp.Payload.Username)
	d.Set("role", resp.Payload.Role)

	// Attempt to find team_id for standard users
	if resp.Payload.Role == 2 {
		paramsTM := team_memberships.NewTeamMembershipListParams()
		respTM, err := client.Client.TeamMemberships.TeamMembershipList(paramsTM, client.AuthInfo)
		if err == nil {
			for _, m := range respTM.Payload {
				if m.UserID == id {
					d.Set("team_id", int(m.TeamID))
					break
				}
			}
		}
	}

	return nil
}

func resourceUserImport(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	client := meta.(*APIClient)

	// Check if the ID is a numeric ID
	if _, err := strconv.ParseInt(d.Id(), 10, 64); err == nil {
		// It's a numeric ID, so just read it
		if err := resourceUserRead(d, meta); err != nil {
			return nil, err
		}
		return []*schema.ResourceData{d}, nil
	}

	// It's not a numeric ID, so treat it as a username
	username := d.Id()
	params := users.NewUserListParams()
	resp, err := client.Client.Users.UserList(params, client.AuthInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to list users for import: %w", err)
	}

	for _, u := range resp.Payload {
		if u.Username == username {
			d.SetId(strconv.FormatInt(u.ID, 10))
			if err := resourceUserRead(d, meta); err != nil {
				return nil, err
			}
			return []*schema.ResourceData{d}, nil
		}
	}

	return nil, fmt.Errorf("user %s not found", username)
}

func resourceUserReadByUsername(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	username := d.Get("username").(string)

	params := users.NewUserListParams()
	resp, err := client.Client.Users.UserList(params, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to list users for lookup: %w", err)
	}

	for _, u := range resp.Payload {
		if u.Username == username {
			d.SetId(strconv.FormatInt(u.ID, 10))
			d.Set("role", u.Role)
			return nil
		}
	}

	return fmt.Errorf("user created but not found in user list")
}

func resourceUserUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	if d.HasChange("password") {
		oldPw, newPw := d.GetChange("password")
		oldPwString := oldPw.(string)
		newPwString := newPw.(string)

		paramsPwd := users.NewUserUpdatePasswordParams()
		paramsPwd.ID = id
		paramsPwd.Body = &models.UsersUserUpdatePasswordPayload{
			Password:    &oldPwString,
			NewPassword: &newPwString,
		}

		_, err := client.Client.Users.UserUpdatePassword(paramsPwd, client.AuthInfo)
		if err != nil {
			return fmt.Errorf("failed to update password: %w", err)
		}
	}

	username := d.Get("username").(string)
	role := int64(d.Get("role").(int))
	useCache := true

	params := users.NewUserUpdateParams()
	params.ID = id
	params.Body = &models.UsersUserUpdatePayload{
		Username: &username,
		Role:     &role,
		UseCache: &useCache,
	}

	_, err := client.Client.Users.UserUpdate(params, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return resourceUserRead(d, meta)
}

func resourceUserDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	// API key deletion is implicit or we should list and delete?
	// Original code tried to delete specific token if api_key_id was present (which wasn't in schema?)
	// But it also had keyID check. The SDK UserDelete will delete the user and their tokens.

	params := users.NewUserDeleteParams()
	params.ID = id

	_, err := client.Client.Users.UserDelete(params, client.AuthInfo)
	if err != nil {
		if _, ok := err.(*users.UserDeleteNotFound); ok {
			return nil
		}
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}
