package internal

import (
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/portainer/client-api-go/v2/pkg/client/users"
	"github.com/portainer/client-api-go/v2/pkg/models"
)

func resourceUserAdmin() *schema.Resource {
	return &schema.Resource{
		Create: resourceUserAdminCreate,
		Read:   resourceUserAdminRead,
		Update: resourceUserAdminUpdate,
		Delete: resourceUserAdminDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		Schema: map[string]*schema.Schema{
			"username": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "admin",
				Description: "Username of the admin account to initialize (defaults to 'admin').",
			},
			"password": {
				Type:        schema.TypeString,
				Required:    true,
				Sensitive:   true,
				Description: "Password for the admin account. Only used during initial bootstrap.",
			},
			"initialized": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the admin user has been initialized via this resource.",
			},
		},
	}
}

func resourceUserAdminCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	username := d.Get("username").(string)
	password := d.Get("password").(string)

	params := users.NewUserAdminInitParams()
	params.Body = &models.UsersAdminInitPayload{
		Username: &username,
		Password: &password,
	}

	resp, err := client.Client.Users.UserAdminInit(params)
	if err != nil {
		// Treat 409 (admin already initialized) as a successful, idempotent create.
		if _, ok := err.(*users.UserAdminInitConflict); ok {
			if d.Id() == "" {
				d.SetId("portainer-admin")
			}
			_ = d.Set("initialized", true)
			return nil
		}
		return fmt.Errorf("failed to initialize admin user: %w", err)
	}

	if resp.Payload.ID != 0 {
		d.SetId(strconv.FormatInt(resp.Payload.ID, 10))
	} else {
		d.SetId("portainer-admin")
	}

	_ = d.Set("initialized", true)

	return nil
}

func resourceUserAdminRead(d *schema.ResourceData, meta interface{}) error {
	d.SetId("")
	return nil
}

func resourceUserAdminUpdate(d *schema.ResourceData, meta interface{}) error {
	d.SetId("")
	return nil
}

func resourceUserAdminDelete(d *schema.ResourceData, meta interface{}) error {
	d.SetId("")
	return nil
}
