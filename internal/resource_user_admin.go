package internal

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/portainer/client-api-go/v2/pkg/client/users"
	"github.com/portainer/client-api-go/v2/pkg/models"
)

func resourceUserAdmin() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceUserAdminCreate,
		ReadContext:   resourceUserAdminRead,
		UpdateContext: resourceUserAdminUpdate,
		DeleteContext: resourceUserAdminDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
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

func resourceUserAdminCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	username := d.Get("username").(string)
	password := d.Get("password").(string)

	ctx, errBody := withErrorCapture(ctx)
	params := users.NewUserAdminInitParams()
	params.SetContext(ctx)
	params.Body = &models.UsersAdminInitPayload{
		Username: &username,
		Password: &password,
	}

	resp, err := client.Client.Users.UserAdminInit(params)
	if err != nil {
		// Treat 409 (admin already initialized) as a successful, idempotent create.
		var conflict *users.UserAdminInitConflict
		if errors.As(err, &conflict) {
			if d.Id() == "" {
				d.SetId("portainer-admin")
			}
			_ = d.Set("initialized", true)
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to initialize admin user: %w", decorateSDKError(err, errBody)))
	}

	if resp.Payload.ID != 0 {
		d.SetId(strconv.FormatInt(resp.Payload.ID, 10))
	} else {
		d.SetId("portainer-admin")
	}

	_ = d.Set("initialized", true)

	return nil
}

func resourceUserAdminRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	d.SetId("")
	return nil
}

func resourceUserAdminUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	d.SetId("")
	return nil
}

func resourceUserAdminDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	d.SetId("")
	return nil
}
