package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// d2kNamespacePrefix is what Portainer puts in front of an instance name to
// get the namespace the d2k install runs in.
const d2kNamespacePrefix = "d2k-"

// d2kInstance is one entry of the POST /endpoints/d2k payload.
type d2kInstance struct {
	Name            string `json:"Name"`
	EnvironmentName string `json:"EnvironmentName,omitempty"`
}

// d2kCreatePayload is the POST /endpoints/d2k body. The endpoint installs a
// batch of instances; the resource always sends exactly one.
type d2kCreatePayload struct {
	ParentEnvironmentID int           `json:"ParentEnvironmentID"`
	Instances           []d2kInstance `json:"Instances"`
	SwarmMode           bool          `json:"SwarmMode"`
	GroupID             int           `json:"GroupID,omitempty"`
	TagIDs              []int         `json:"TagIDs,omitempty"`
}

// d2kCreateResult is the per-instance outcome POST /endpoints/d2k reports:
// an instance that fails does not fail the request, so its Error has to be
// checked on its own.
type d2kCreateResult struct {
	Name        string `json:"Name"`
	Namespace   string `json:"Namespace"`
	Error       string `json:"Error"`
	Environment *struct {
		ID int `json:"Id"`
	} `json:"Environment"`
}

// d2kEndpoint is the part of GET /endpoints/{id} a D2K environment is read
// from. D2K is only included for D2K environments.
type d2kEndpoint struct {
	ID      int     `json:"Id"`
	Name    string  `json:"Name"`
	GroupID int     `json:"GroupId"`
	TagIds  []int64 `json:"TagIds"`
	D2K     *struct {
		Namespace           string `json:"Namespace"`
		ParentEnvironmentID int    `json:"ParentEnvironmentID"`
		SwarmMode           bool   `json:"SwarmMode"`
	} `json:"D2K"`
}

func resourceD2KEnvironment() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceD2KEnvironmentCreate,
		ReadContext:   resourceD2KEnvironmentRead,
		UpdateContext: resourceD2KEnvironmentUpdate,
		DeleteContext: resourceD2KEnvironmentDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"parent_environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the Kubernetes environment d2k is installed in. It must be connected through an Edge Agent 3.0.0 or later in tunnel (standard) mode.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of the d2k install. It runs in the namespace `d2k-<name>`, which is why it cannot be changed in place.",
			},
			"environment_name": {
				Type:     schema.TypeString,
				Optional: true,
				// Portainer names the environment after the install when no
				// name is given, so the name it picked is read back.
				Computed:    true,
				Description: "Name of the D2K environment in Portainer. Defaults to `name`. Can be changed in place.",
			},
			"swarm_mode": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				ForceNew:    true,
				Description: "Whether the install emulates a Docker Swarm manager rather than a standalone Docker engine, which adds services, configs, secrets and Swarm stacks. Fixed at install time.",
			},
			"group_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     1,
				Description: "ID of the Portainer endpoint group of the D2K environment. Default is 1 (Unassigned).",
			},
			"tag_ids": {
				Type:        schema.TypeList,
				Optional:    true,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "List of tag IDs to assign to the D2K environment.",
			},
			// Computed attributes
			"namespace": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Kubernetes namespace of the parent environment the d2k install runs in.",
			},
			"environment_id": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Identifier of the D2K environment as a number (the same value as `id`), for the `endpoint_id` of Docker resources managed through it.",
			},
		},
	}
}

func resourceD2KEnvironmentCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	name := d.Get("name").(string)
	payload := d2kCreatePayload{
		ParentEnvironmentID: d.Get("parent_environment_id").(int),
		Instances: []d2kInstance{{
			Name:            name,
			EnvironmentName: d.Get("environment_name").(string),
		}},
		SwarmMode: d.Get("swarm_mode").(bool),
		GroupID:   d.Get("group_id").(int),
		TagIDs:    toIntSlice(d.Get("tag_ids").([]interface{})),
	}

	var results []d2kCreateResult
	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/endpoints/d2k", payload, &results); err != nil {
		err = explainUnsupported(ctx, client, err, "portainer_d2k_environment", requiresBE30)
		return diag.FromErr(fmt.Errorf("failed to create D2K environment: %w", err))
	}

	var result *d2kCreateResult
	for i := range results {
		if results[i].Name == name {
			result = &results[i]
			break
		}
	}
	if result == nil {
		return diag.Errorf("failed to create D2K environment %q: Portainer reported no result for it", name)
	}

	// An environment that was made despite an error is kept in state, so
	// Terraform marks it tainted and the next apply removes it rather than
	// leaving it behind.
	if result.Environment != nil && result.Environment.ID != 0 {
		d.SetId(strconv.Itoa(result.Environment.ID))
	}
	if result.Error != "" {
		return diag.Errorf("failed to create D2K environment %q: %s", name, result.Error)
	}
	if d.Id() == "" {
		return diag.Errorf("failed to create D2K environment %q: Portainer returned no environment", name)
	}

	return resourceD2KEnvironmentRead(ctx, d, meta)
}

func resourceD2KEnvironmentRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	var env d2kEndpoint
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/endpoints/%s", client.Endpoint, d.Id()), nil, &env); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read D2K environment: %w", err))
	}
	if env.D2K == nil {
		return diag.Errorf("environment %s is not a D2K environment", d.Id())
	}

	tagIDs := make([]int, 0, len(env.TagIds))
	for _, tid := range env.TagIds {
		tagIDs = append(tagIDs, int(tid))
	}

	if err := setFields(d, map[string]interface{}{
		"parent_environment_id": env.D2K.ParentEnvironmentID,
		// The install name is not stored on its own: it is the namespace
		// without the prefix Portainer added, which also covers an import.
		"name":             strings.TrimPrefix(env.D2K.Namespace, d2kNamespacePrefix),
		"namespace":        env.D2K.Namespace,
		"swarm_mode":       env.D2K.SwarmMode,
		"environment_name": env.Name,
		"group_id":         env.GroupID,
		"tag_ids":          tagIDs,
		"environment_id":   env.ID,
	}); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

// resourceD2KEnvironmentUpdate changes what PUT /endpoints/{id} can change on
// a D2K environment: its name, group and tags. Everything else is fixed by
// the d2k install and forces a new resource.
func resourceD2KEnvironmentUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	body := environmentUpdateBody{
		Name:    d.Get("environment_name").(string),
		GroupID: d.Get("group_id").(int),
	}
	// An empty list clears the tags, but only when tag_ids itself changed, so
	// tags assigned outside Terraform survive when the attribute is not set.
	if tags := toIntSlice(d.Get("tag_ids").([]interface{})); len(tags) > 0 || d.HasChange("tag_ids") {
		body.TagIDs = tags
	}

	if err := doJSON(ctx, client, http.MethodPut, fmt.Sprintf("%s/endpoints/%s", client.Endpoint, d.Id()), body, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update D2K environment: %w", err))
	}

	return resourceD2KEnvironmentRead(ctx, d, meta)
}

// resourceD2KEnvironmentDelete removes the D2K environment from Portainer.
// Portainer leaves the d2k install and its workloads running in the cluster.
func resourceD2KEnvironmentDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	if err := doJSON(ctx, client, http.MethodDelete, fmt.Sprintf("%s/endpoints/%s", client.Endpoint, d.Id()), nil, nil); err != nil {
		if isAPINotFound(err) {
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to delete D2K environment: %w", err))
	}

	return nil
}
