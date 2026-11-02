package internal

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"
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
			"setup_token": {
				Type:      schema.TypeString,
				Optional:  true,
				Sensitive: true,
				// A bootstrap credential only the init call needs, so it is
				// never kept in state.
				WriteOnly:   true,
				Description: "Write-only value sent as the `X-Setup-Token` header (never stored in Terraform state; requires Terraform 1.11 or later). Portainer 2.45 and later require it to initialize a fresh instance unless started with `--no-setup-token`; the token is printed in the Portainer log (`setup_token=...`) or set with `--setup-token` / `PORTAINER_SETUP_TOKEN`.",
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

	// Ask first instead of relying on 409 from the init call: Portainer 2.45
	// checks the setup token before it looks for an administrator, so until
	// its next restart an initialized instance answers 403 to a call without
	// the token.
	exists, err := adminExists(ctx, client)
	if err != nil {
		return diag.FromErr(err)
	}
	if exists {
		d.SetId("portainer-admin")
		return diag.FromErr(d.Set("initialized", true))
	}

	ctx, errBody := withErrorCapture(ctx)
	params := users.NewUserAdminInitParams()
	params.SetContext(ctx)
	params.Body = &models.UsersAdminInitPayload{
		Username: &username,
		Password: &password,
	}

	var opts []users.ClientOption
	if !d.GetRawConfig().IsNull() {
		token, err := writeOnlyString(d, "setup_token")
		if err != nil {
			return diag.FromErr(err)
		}
		if token != "" {
			opts = append(opts, withHeader("X-Setup-Token", token))
		}
	}

	resp, err := client.Client.Users.UserAdminInit(params, opts...)
	if err != nil {
		// Treat 409 (admin already initialized) as a successful, idempotent create.
		var conflict *users.UserAdminInitConflict
		if errors.As(err, &conflict) {
			if d.Id() == "" {
				d.SetId("portainer-admin")
			}
			if err := d.Set("initialized", true); err != nil {
				return diag.FromErr(err)
			}
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to initialize admin user: %w", decorateSDKError(err, errBody)))
	}

	if resp.Payload.ID != 0 {
		d.SetId(strconv.FormatInt(resp.Payload.ID, 10))
	} else {
		d.SetId("portainer-admin")
	}

	if err := d.Set("initialized", true); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

// withHeader adds a request header to a single SDK call.
func withHeader(name, value string) users.ClientOption {
	return func(op *runtime.ClientOperation) {
		params := op.Params
		op.Params = runtime.ClientRequestWriterFunc(func(r runtime.ClientRequest, reg strfmt.Registry) error {
			if err := params.WriteToRequest(r, reg); err != nil {
				return err
			}
			return r.SetHeaderParam(name, value)
		})
	}
}

// resourceUserAdminRead keeps the resource once the instance has an
// administrator, so the plan settles after the first apply instead of
// proposing the bootstrap again on every run. Only an instance that lost its
// administrator (a reset or a fresh data volume) drops the resource, which
// plans the initialization again.
func resourceUserAdminRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	exists, err := adminExists(ctx, meta.(*APIClient))
	if err != nil {
		return diag.FromErr(err)
	}
	if !exists {
		d.SetId("")
		return nil
	}
	return diag.FromErr(d.Set("initialized", true))
}

// adminExists asks the public GET /users/admin/check whether the instance has
// an administrator.
func adminExists(ctx context.Context, client *APIClient) (bool, error) {
	ctx, errBody := withErrorCapture(ctx)
	params := users.NewUserAdminCheckParams()
	params.SetContext(ctx)
	if _, err := client.Client.Users.UserAdminCheck(params); err != nil {
		var notFound *users.UserAdminCheckNotFound
		if errors.As(err, &notFound) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check for an admin user: %w", decorateSDKError(err, errBody))
	}
	return true, nil
}

// resourceUserAdminUpdate stores the new arguments without calling Portainer:
// the init API runs once, so a changed username or password cannot be applied
// to an initialized instance.
func resourceUserAdminUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return nil
}

func resourceUserAdminDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	d.SetId("")
	return nil
}
