package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// resourceKubernetesPodSecurityRule manages the OPA-backed pod security rule
// of a Kubernetes environment.
//
// Portainer always has a rule for a cluster - its own UI creates a default one
// on first view - so this resource adopts and configures that rule rather than
// creating one. Destroying it switches the rule off rather than deleting it,
// which is the only "absent" state the endpoint has.
func resourceKubernetesPodSecurityRule() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceKubernetesPodSecurityRuleWrite,
		ReadContext:   resourceKubernetesPodSecurityRuleRead,
		UpdateContext: resourceKubernetesPodSecurityRuleWrite,
		DeleteContext: resourceKubernetesPodSecurityRuleDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the Kubernetes environment the rule applies to.",
			},
			"enabled": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "Whether the pod security rule is enforced at all. With this off, the individual sections below are stored but do nothing.",
			},
			"privileged_containers": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to block privileged containers.",
			},
			"allow_privilege_escalation": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to block containers that may escalate their privileges.",
			},
			"host_namespaces": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to block pods that join the host's namespaces.",
			},
			"read_only_root_filesystem": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to require a read-only root filesystem.",
			},
			"restrict_default_namespace": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to stop workloads being deployed into the `default` namespace.",
			},
			"restrict_secrets": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to restrict access to secrets.",
			},
			"allow_proc_mount": {
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Description: "Which proc mount types are permitted.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"enabled": {
							Type:        schema.TypeBool,
							Optional:    true,
							Default:     true,
							Description: "Whether this section is enforced.",
						},
						"proc_mount_type": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "The proc mount type to permit, for example `Default` or `Unmasked`.",
						},
					},
				},
			},
			"allow_flex_volumes": {
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Description: "Which FlexVolume drivers are permitted.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"enabled": {
							Type:        schema.TypeBool,
							Optional:    true,
							Default:     true,
							Description: "Whether this section is enforced.",
						},
						"allowed_volumes": {
							Type:        schema.TypeList,
							Optional:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "FlexVolume drivers a pod may use.",
						},
					},
				},
			},
			"app_armor": {
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Description: "Which AppArmor profiles are permitted.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"enabled": {
							Type:        schema.TypeBool,
							Optional:    true,
							Default:     true,
							Description: "Whether this section is enforced.",
						},
						"types": {
							Type:        schema.TypeList,
							Optional:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "AppArmor profiles a pod may use.",
						},
					},
				},
			},
			"capabilities": {
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Description: "Which Linux capabilities a container may hold.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"enabled": {
							Type:        schema.TypeBool,
							Optional:    true,
							Default:     true,
							Description: "Whether this section is enforced.",
						},
						"allowed": {
							Type:        schema.TypeList,
							Optional:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Capabilities a container may add.",
						},
						"required_drop": {
							Type:        schema.TypeList,
							Optional:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Capabilities every container has to drop.",
						},
					},
				},
			},
			"forbidden_sysctls": {
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Description: "Which sysctls a pod may not set.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"enabled": {
							Type:        schema.TypeBool,
							Optional:    true,
							Default:     true,
							Description: "Whether this section is enforced.",
						},
						"sysctls": {
							Type:     schema.TypeList,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
							// Portainer stores this list under a field named
							// requiredDropCapabilities, which is a misnomer in
							// its own schema rather than something to copy here.
							Description: "Sysctls a pod may not set.",
						},
					},
				},
			},
			"host_filesystem": {
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Description: "Which host paths a pod may mount.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"enabled": {
							Type:        schema.TypeBool,
							Optional:    true,
							Default:     true,
							Description: "Whether this section is enforced.",
						},
						"allowed_path": {
							Type:        schema.TypeList,
							Optional:    true,
							Description: "Host paths a pod may mount. Repeatable.",
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"path_prefix": {
										Type:        schema.TypeString,
										Required:    true,
										Description: "Path prefix that may be mounted.",
									},
									"readonly": {
										Type:        schema.TypeBool,
										Optional:    true,
										Default:     false,
										Description: "Whether the mount has to be read-only.",
									},
								},
							},
						},
					},
				},
			},
			"host_ports": {
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Description: "Which host ports a pod may bind, and whether host networking is allowed at all.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"enabled": {
							Type:        schema.TypeBool,
							Optional:    true,
							Default:     true,
							Description: "Whether this section is enforced.",
						},
						"host_network": {
							Type:        schema.TypeBool,
							Optional:    true,
							Default:     false,
							Description: "Whether a pod may use the host's network namespace.",
						},
						"min": {
							Type:        schema.TypeInt,
							Optional:    true,
							Description: "Lowest host port a pod may bind.",
						},
						"max": {
							Type:        schema.TypeInt,
							Optional:    true,
							Description: "Highest host port a pod may bind.",
						},
					},
				},
			},
			"sec_comp": {
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Description: "Which seccomp profiles are permitted.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"enabled": {
							Type:        schema.TypeBool,
							Optional:    true,
							Default:     true,
							Description: "Whether this section is enforced.",
						},
						"types": {
							Type:        schema.TypeList,
							Optional:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Seccomp profiles a pod may use.",
						},
					},
				},
			},
			"selinux": {
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Description: "Which SELinux contexts are permitted.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"enabled": {
							Type:        schema.TypeBool,
							Optional:    true,
							Default:     true,
							Description: "Whether this section is enforced.",
						},
						"allowed_context": {
							Type:        schema.TypeList,
							Optional:    true,
							Description: "SELinux contexts a pod may run under. Repeatable.",
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"user": {
										Type:        schema.TypeString,
										Optional:    true,
										Description: "SELinux user.",
									},
									"role": {
										Type:        schema.TypeString,
										Optional:    true,
										Description: "SELinux role.",
									},
									"type": {
										Type:        schema.TypeString,
										Optional:    true,
										Description: "SELinux type.",
									},
									"level": {
										Type:        schema.TypeString,
										Optional:    true,
										Description: "SELinux level.",
									},
								},
							},
						},
					},
				},
			},
			"volume_types": {
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Description: "Which volume types a pod may use.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"enabled": {
							Type:        schema.TypeBool,
							Optional:    true,
							Default:     true,
							Description: "Whether this section is enforced.",
						},
						"allowed_types": {
							Type:        schema.TypeList,
							Optional:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Volume types a pod may use, for example `configMap` or `emptyDir`.",
						},
					},
				},
			},
			"users": {
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Description: "Which user and group identities a pod may run as.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"enabled": {
							Type:        schema.TypeBool,
							Optional:    true,
							Default:     true,
							Description: "Whether this section is enforced.",
						},
						"run_as_user": {
							Type:        schema.TypeList,
							Optional:    true,
							MaxItems:    1,
							Description: "Strategy and ranges for the user a container runs as.",
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"type": {
										Type:        schema.TypeString,
										Optional:    true,
										Description: "Strategy, for example `MustRunAs` or `RunAsAny`.",
									},
									"id_range": {
										Type:        schema.TypeList,
										Optional:    true,
										Description: "Permitted identifier ranges. Repeatable.",
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"min": {
													Type:        schema.TypeInt,
													Required:    true,
													Description: "Lowest identifier in the range.",
												},
												"max": {
													Type:        schema.TypeInt,
													Required:    true,
													Description: "Highest identifier in the range.",
												},
											},
										},
									},
								},
							},
						},
						"run_as_group": {
							Type:        schema.TypeList,
							Optional:    true,
							MaxItems:    1,
							Description: "Strategy and ranges for the group a container runs as.",
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"type": {
										Type:        schema.TypeString,
										Optional:    true,
										Description: "Strategy, for example `MustRunAs` or `RunAsAny`.",
									},
									"id_range": {
										Type:        schema.TypeList,
										Optional:    true,
										Description: "Permitted identifier ranges. Repeatable.",
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"min": {
													Type:        schema.TypeInt,
													Required:    true,
													Description: "Lowest identifier in the range.",
												},
												"max": {
													Type:        schema.TypeInt,
													Required:    true,
													Description: "Highest identifier in the range.",
												},
											},
										},
									},
								},
							},
						},
						"fs_groups": {
							Type:        schema.TypeList,
							Optional:    true,
							MaxItems:    1,
							Description: "Strategy and ranges for the pod's filesystem groups.",
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"type": {
										Type:        schema.TypeString,
										Optional:    true,
										Description: "Strategy, for example `MustRunAs` or `RunAsAny`.",
									},
									"id_range": {
										Type:        schema.TypeList,
										Optional:    true,
										Description: "Permitted identifier ranges. Repeatable.",
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"min": {
													Type:        schema.TypeInt,
													Required:    true,
													Description: "Lowest identifier in the range.",
												},
												"max": {
													Type:        schema.TypeInt,
													Required:    true,
													Description: "Highest identifier in the range.",
												},
											},
										},
									},
								},
							},
						},
						"supplemental_groups": {
							Type:        schema.TypeList,
							Optional:    true,
							MaxItems:    1,
							Description: "Strategy and ranges for the pod's supplemental groups.",
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"type": {
										Type:        schema.TypeString,
										Optional:    true,
										Description: "Strategy, for example `MustRunAs` or `RunAsAny`.",
									},
									"id_range": {
										Type:        schema.TypeList,
										Optional:    true,
										Description: "Permitted identifier ranges. Repeatable.",
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"min": {
													Type:        schema.TypeInt,
													Required:    true,
													Description: "Lowest identifier in the range.",
												},
												"max": {
													Type:        schema.TypeInt,
													Required:    true,
													Description: "Highest identifier in the range.",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// podSecurityBlock reads a MaxItems-1 block, returning the block's contents
// and whether it was present at all.
func podSecurityBlock(d *schema.ResourceData, key string) (map[string]interface{}, bool) {
	list, _ := d.Get(key).([]interface{})
	if len(list) == 0 {
		return nil, false
	}
	block, ok := list[0].(map[string]interface{})
	return block, ok
}

// podSecuritySection builds the {"enabled": ...} object every section shares.
func podSecuritySection(block map[string]interface{}) map[string]interface{} {
	enabled, _ := block["enabled"].(bool)
	return map[string]interface{}{"enabled": enabled}
}

// podSecurityStrategy encodes a run-as strategy with its identifier ranges.
func podSecurityStrategy(raw interface{}) (map[string]interface{}, bool) {
	list, _ := raw.([]interface{})
	if len(list) == 0 {
		return nil, false
	}
	block, ok := list[0].(map[string]interface{})
	if !ok {
		return nil, false
	}
	strategy := map[string]interface{}{}
	if v, ok := block["type"].(string); ok && v != "" {
		strategy["type"] = v
	}
	ranges, _ := block["id_range"].([]interface{})
	encoded := make([]map[string]interface{}, 0, len(ranges))
	for _, item := range ranges {
		entry, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		encoded = append(encoded, map[string]interface{}{
			"min": entry["min"], "max": entry["max"],
		})
	}
	if len(encoded) > 0 {
		strategy["idrange"] = encoded
	}
	return strategy, true
}

func podSecurityRulePayload(d *schema.ResourceData) map[string]interface{} {
	payload := map[string]interface{}{
		"endPointID": d.Get("endpoint_id").(int),
		"enabled":    d.Get("enabled").(bool),
	}

	// The plain switches, which Portainer stores as one-field objects.
	for field, key := range map[string]string{
		"privileged_containers":      "privilegedContainers",
		"allow_privilege_escalation": "allowPrivilegeEscalation",
		"host_namespaces":            "hostNamespaces",
		"read_only_root_filesystem":  "readOnlyRootFileSystem",
		"restrict_default_namespace": "restrictDefaultNamespace",
		"restrict_secrets":           "restrictSecrets",
	} {
		payload[key] = map[string]interface{}{"enabled": d.Get(field).(bool)}
	}

	if block, ok := podSecurityBlock(d, "allow_proc_mount"); ok {
		section := podSecuritySection(block)
		if v, ok := block["proc_mount_type"].(string); ok && v != "" {
			section["procMountType"] = v
		}
		payload["allowProcMount"] = section
	}
	if block, ok := podSecurityBlock(d, "allow_flex_volumes"); ok {
		section := podSecuritySection(block)
		section["allowedVolumes"] = block["allowed_volumes"]
		payload["allowFlexVolumes"] = section
	}
	if block, ok := podSecurityBlock(d, "app_armor"); ok {
		section := podSecuritySection(block)
		section["AppArmorType"] = block["types"]
		payload["appArmor"] = section
	}
	if block, ok := podSecurityBlock(d, "capabilities"); ok {
		section := podSecuritySection(block)
		section["allowedCapabilities"] = block["allowed"]
		section["requiredDropCapabilities"] = block["required_drop"]
		payload["capabilities"] = section
	}
	if block, ok := podSecurityBlock(d, "forbidden_sysctls"); ok {
		section := podSecuritySection(block)
		// Portainer stores the sysctl list under requiredDropCapabilities,
		// which is a misnomer in its own schema.
		section["requiredDropCapabilities"] = block["sysctls"]
		payload["forbiddenSysctlsList"] = section
	}
	if block, ok := podSecurityBlock(d, "host_filesystem"); ok {
		section := podSecuritySection(block)
		paths, _ := block["allowed_path"].([]interface{})
		encoded := make([]map[string]interface{}, 0, len(paths))
		for _, item := range paths {
			entry, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			encoded = append(encoded, map[string]interface{}{
				"pathPrefix": entry["path_prefix"], "readonly": entry["readonly"],
			})
		}
		section["allowedPaths"] = encoded
		payload["hostFilesystem"] = section
	}
	if block, ok := podSecurityBlock(d, "host_ports"); ok {
		section := podSecuritySection(block)
		section["hostNetwork"] = block["host_network"]
		section["min"] = block["min"]
		section["max"] = block["max"]
		payload["hostPorts"] = section
	}
	if block, ok := podSecurityBlock(d, "sec_comp"); ok {
		section := podSecuritySection(block)
		section["secCompType"] = block["types"]
		payload["secComp"] = section
	}
	if block, ok := podSecurityBlock(d, "selinux"); ok {
		section := podSecuritySection(block)
		contexts, _ := block["allowed_context"].([]interface{})
		encoded := make([]map[string]interface{}, 0, len(contexts))
		for _, item := range contexts {
			entry, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			encoded = append(encoded, map[string]interface{}{
				"user": entry["user"], "role": entry["role"],
				"type": entry["type"], "level": entry["level"],
			})
		}
		section["allowedCapabilities"] = encoded
		payload["selinux"] = section
	}
	if block, ok := podSecurityBlock(d, "volume_types"); ok {
		section := podSecuritySection(block)
		section["allowedTypes"] = block["allowed_types"]
		payload["volumeTypes"] = section
	}
	if block, ok := podSecurityBlock(d, "users"); ok {
		section := podSecuritySection(block)
		for field, key := range map[string]string{
			"run_as_user":         "runAsUser",
			"run_as_group":        "runAsGroup",
			"fs_groups":           "fsGroups",
			"supplemental_groups": "supplementalGroups",
		} {
			if strategy, ok := podSecurityStrategy(block[field]); ok {
				section[key] = strategy
			}
		}
		payload["users"] = section
	}

	return payload
}

func resourceKubernetesPodSecurityRuleWrite(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)

	ruleURL := fmt.Sprintf("%s/kubernetes/%d/opa", client.Endpoint, endpointID)
	if err := doJSON(ctx, client, http.MethodPut, ruleURL, podSecurityRulePayload(d), nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update the pod security rule of environment %d: %w", endpointID, err))
	}

	d.SetId(fmt.Sprintf("%d", endpointID))
	return resourceKubernetesPodSecurityRuleRead(ctx, d, meta)
}

func resourceKubernetesPodSecurityRuleRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	var endpointID int
	if _, err := fmt.Sscanf(d.Id(), "%d", &endpointID); err != nil {
		return diag.FromErr(fmt.Errorf("the resource ID must be an environment identifier, got %q", d.Id()))
	}

	// Only the switches are read back. The individual sections are lists of
	// permitted values that Portainer normalises and reorders, so reading them
	// into state would turn a stable configuration into a churning plan; what
	// they contain is driven entirely by the configuration instead.
	var rule struct {
		Enabled              bool `json:"enabled"`
		EndpointID           int  `json:"endPointID"`
		PrivilegedContainers struct {
			Enabled bool `json:"enabled"`
		} `json:"privilegedContainers"`
		AllowPrivilegeEscalation struct {
			Enabled bool `json:"enabled"`
		} `json:"allowPrivilegeEscalation"`
		HostNamespaces struct {
			Enabled bool `json:"enabled"`
		} `json:"hostNamespaces"`
		ReadOnlyRootFileSystem struct {
			Enabled bool `json:"enabled"`
		} `json:"readOnlyRootFileSystem"`
		RestrictDefaultNamespace struct {
			Enabled bool `json:"enabled"`
		} `json:"restrictDefaultNamespace"`
		RestrictSecrets struct {
			Enabled bool `json:"enabled"`
		} `json:"restrictSecrets"`
	}

	ruleURL := fmt.Sprintf("%s/kubernetes/%d/opa", client.Endpoint, endpointID)
	if err := doJSON(ctx, client, http.MethodGet, ruleURL, nil, &rule); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read the pod security rule of environment %d: %w", endpointID, err))
	}

	if err := setFields(d, map[string]interface{}{
		"endpoint_id":                endpointID,
		"enabled":                    rule.Enabled,
		"privileged_containers":      rule.PrivilegedContainers.Enabled,
		"allow_privilege_escalation": rule.AllowPrivilegeEscalation.Enabled,
		"host_namespaces":            rule.HostNamespaces.Enabled,
		"read_only_root_filesystem":  rule.ReadOnlyRootFileSystem.Enabled,
		"restrict_default_namespace": rule.RestrictDefaultNamespace.Enabled,
		"restrict_secrets":           rule.RestrictSecrets.Enabled,
	}); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceKubernetesPodSecurityRuleDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	var endpointID int
	if _, err := fmt.Sscanf(d.Id(), "%d", &endpointID); err != nil {
		return diag.FromErr(fmt.Errorf("the resource ID must be an environment identifier, got %q", d.Id()))
	}

	// Portainer has no endpoint to remove a rule - a cluster always has one -
	// so destroying the resource switches it off. That is the only "absent"
	// state available, and leaving an enforced policy behind for a rule
	// Terraform no longer manages would be worse.
	payload := map[string]interface{}{"endPointID": endpointID, "enabled": false}
	ruleURL := fmt.Sprintf("%s/kubernetes/%d/opa", client.Endpoint, endpointID)
	if err := doJSON(ctx, client, http.MethodPut, ruleURL, payload, nil); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to disable the pod security rule of environment %d: %w", endpointID, err))
	}

	d.SetId("")
	return nil
}
