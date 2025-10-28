package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceDeploy() *schema.Resource {
	return &schema.Resource{
		Create: resourceDeployCreate,
		Read:   resourceDeployRead,   // stateless
		Delete: resourceDeployDelete, // stateless
		Update: nil,
		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},
			"stack_name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"stack_env_var": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of stack environment variable to update.",
			},
			"revision": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Target image tag/revision to set on services and optionally on stack ENV in stack_env_var.",
			},
			"services_list": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Comma-separated list of service names (without stack prefix).",
			},
			"update_revision": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				ForceNew:    true,
				Description: "If true, also update stack ENV variable in stack_env_var to the provided revision.",
			},
			"force_update": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				ForceNew:    true,
				Description: "If true, call Portainer forceupdateservice endpoint for each updated service (after optional wait).",
			},
			"wait": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     30,
				ForceNew:    true,
				Description: "Seconds to wait before force-updating a service (only when force_update = true).",
			},
			"output": {
				Type:     schema.TypeString,
				Computed: true,
				ForceNew: true,
			},
		},
	}
}

func resourceDeployCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	stackEnvVar := d.Get("stack_env_var").(string)
	endpointID := d.Get("endpoint_id").(int)
	stackName := d.Get("stack_name").(string)
	revision := d.Get("revision").(string)
	updateRevision := d.Get("update_revision").(bool)
	forceUpdate := d.Get("force_update").(bool)
	wait := d.Get("wait").(int)

	servicesListRaw := d.Get("services_list").(string)
	trimmed := strings.TrimSpace(servicesListRaw)
	if trimmed == "" {
		return fmt.Errorf("services_list must not be empty")
	}
	shortServices := splitAndTrimCSV(trimmed)
	fullServices := make([]string, 0, len(shortServices))
	for _, s := range shortServices {
		fullServices = append(fullServices, fmt.Sprintf("%s_%s", stackName, s))
	}

	var out strings.Builder

	// Detect swarm
	swURL := fmt.Sprintf("%s/endpoints/%d/docker/swarm", client.Endpoint, endpointID)
	swBody, swCode, err := apiGETWithCode(swURL, client.APIKey, client)
	if err != nil {
		return err
	}
	isSwarm := swCode == 200 && bytes.Contains(swBody, []byte(`"ID"`))

	if isSwarm {
		out.WriteString("Docker Swarm detected — using swarm update logic.\n")

		// Parse swarm for ID
		var swarm struct {
			ID string `json:"ID"`
		}
		_ = json.Unmarshal(swBody, &swarm)

		// Get stacks with SwarmID filter and find our stack
		stacksURL := fmt.Sprintf("%s/stacks?filters=%s", client.Endpoint, url.QueryEscape(fmt.Sprintf(`{"SwarmID": "%s"}`, swarm.ID)))
		stacksBytes, err := apiGET(stacksURL, client.APIKey, client)
		if err != nil {
			return fmt.Errorf("failed to query stacks: %w", err)
		}
		var stacks []struct {
			ID   int    `json:"Id"`
			Name string `json:"Name"`
			Env  []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"Env"`
		}
		if err := json.Unmarshal(stacksBytes, &stacks); err != nil {
			return fmt.Errorf("failed to parse stacks response: %w", err)
		}
		var stackSpec *struct {
			ID   int    `json:"Id"`
			Name string `json:"Name"`
			Env  []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"Env"`
		}
		for i := range stacks {
			if stacks[i].Name == stackName {
				stackSpec = &stacks[i]
				break
			}
		}
		if stackSpec == nil {
			return fmt.Errorf("stack %q not found in swarm", stackName)
		}

		// Query services by stack prefix
		servicesURL := fmt.Sprintf("%s/endpoints/%d/docker/services?filters=%s",
			client.Endpoint, endpointID, url.QueryEscape(fmt.Sprintf(`{"name":{"%s":true}}`, stackName)))
		servicesBytes, err := apiGET(servicesURL, client.APIKey, client)
		if err != nil {
			return fmt.Errorf("failed to query services: %w", err)
		}
		var services []map[string]interface{}
		if err := json.Unmarshal(servicesBytes, &services); err != nil {
			return fmt.Errorf("failed to parse services response: %w", err)
		}

		updatedAny := false
		imgTagRe := regexp.MustCompile(`^(.+?):([^@]+)(?:@.*)?$`)

		for _, svc := range services {
			// name
			spec := mustMap(svc["Spec"])
			svcName, _ := spec["Name"].(string)
			if !contains(fullServices, svcName) {
				continue
			}

			taskTpl := mustMap(spec["TaskTemplate"])
			containerSpec := mustMap(taskTpl["ContainerSpec"])
			imageStr, _ := containerSpec["Image"].(string)

			// parse current image tag
			currentTag := ""
			imageRepo := ""
			if m := imgTagRe.FindStringSubmatch(imageStr); len(m) == 3 {
				imageRepo = m[1]
				currentTag = m[2]
			}

			if currentTag == revision {
				out.WriteString(fmt.Sprintf("Service %q already uses revision %q — skip.\n", svcName, revision))
				continue
			}

			// set new image tag and label
			newImage := fmt.Sprintf("%s:%s", imageRepo, revision)
			containerSpec["Image"] = newImage

			labels := mustMap(spec["Labels"])
			labels["com.docker.stack.image"] = newImage

			// version index for update
			version := mustMap(svc["Version"])
			index := fmt.Sprintf("%.0f", version["Index"])

			postURL := fmt.Sprintf("%s/endpoints/%d/docker/services/%s/update?version=%s",
				client.Endpoint, endpointID, url.PathEscape(svcName), index)
			postBody, _ := json.Marshal(spec)
			respBytes, code, err := apiPOSTWithCode(postURL, client.APIKey, client, postBody)
			if err != nil {
				return fmt.Errorf("service %s update request failed: %w", svcName, err)
			}
			if code != 200 {
				return fmt.Errorf("service %s update failed: status %d, body: %s", svcName, code, string(respBytes))
			}
			updatedAny = true
			out.WriteString(fmt.Sprintf("Service %q updated to %q\n", svcName, newImage))

			// check warnings
			var updOut struct {
				Warnings interface{} `json:"Warnings"`
			}
			_ = json.Unmarshal(respBytes, &updOut)
			if updOut.Warnings != nil && fmt.Sprint(updOut.Warnings) != "<nil>" && fmt.Sprint(updOut.Warnings) != "None" {
				out.WriteString(fmt.Sprintf("WARN: service update returned warnings: %v\n", updOut.Warnings))
			}

			// optional force update
			if forceUpdate {
				if wait > 0 {
					time.Sleep(time.Duration(wait) * time.Second)
				}
				forceURL := fmt.Sprintf("%s/endpoints/%d/forceupdateservice", client.Endpoint, endpointID)
				forcePayload := map[string]interface{}{
					"pullImage": true,
					"serviceID": svcName,
				}
				body, _ := json.Marshal(forcePayload)
				resp, code, err := apiPUTWithCode(forceURL, client.APIKey, client, body)
				if err != nil || code != 200 {
					out.WriteString(fmt.Sprintf("Force update of %q failed (status %d): %s\n", svcName, code, string(resp)))
				} else {
					out.WriteString(fmt.Sprintf("Force update of %q succeeded\n", svcName))
				}
			}
		}

		if !updatedAny {
			out.WriteString(fmt.Sprintf("No update needed. All requested services already use revision %q.\n", revision))
		}

		// Update stack_env_var env on stack (if requested)
		if updateRevision && stackSpec != nil {
			// GET stack file
			sfURL := fmt.Sprintf("%s/stacks/%d/file", client.Endpoint, stackSpec.ID)
			sfBytes, code, err := apiGETWithCode(sfURL, client.APIKey, client)
			if err != nil || code != 200 {
				return fmt.Errorf("failed to read stack file (%d): %w", code, err)
			}
			var sf struct {
				StackFileContent string `json:"StackFileContent"`
			}
			if err := json.Unmarshal(sfBytes, &sf); err != nil {
				return fmt.Errorf("failed to parse stack file content: %w", err)
			}

			// ensure/update stack_env_var in Env
			env := stackSpec.Env
			found := false
			for i := range env {
				if env[i].Name == stackEnvVar {
					if env[i].Value != revision {
						env[i].Value = revision
					}
					found = true
					break
				}
			}
			if !found {
				env = append(env, struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				}{Name: stackEnvVar, Value: revision})
			}

			// Build payload
			type envKV struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			}
			envPayload := make([]envKV, 0, len(env))
			for _, kv := range env {
				envPayload = append(envPayload, envKV{Name: kv.Name, Value: kv.Value})
			}
			putBody := map[string]interface{}{
				"StackFileContent": sf.StackFileContent,
				"Prune":            true,
				"Env":              envPayload,
			}
			body, _ := json.Marshal(putBody)
			updURL := fmt.Sprintf("%s/stacks/%d?endpointId=%d", client.Endpoint, stackSpec.ID, endpointID)
			resp, code, err := apiPUTWithCode(updURL, client.APIKey, client, body)
			if err != nil || code != 200 {
				return fmt.Errorf("failed to update stack %s (status %d): %s", stackEnvVar, code, string(resp))
			}
			out.WriteString(fmt.Sprintf("Stack %q %s updated to %q\n", stackName, stackEnvVar, revision))
		}

	} else {
		// Standalone
		out.WriteString("Docker Standalone detected — using standalone stack update logic.\n")

		// list stacks and find by name
		stacksURL := fmt.Sprintf("%s/stacks", client.Endpoint)
		stacksBytes, err := apiGET(stacksURL, client.APIKey, client)
		if err != nil {
			return fmt.Errorf("failed to list stacks: %w", err)
		}
		var stacks []struct {
			ID   int    `json:"Id"`
			Name string `json:"Name"`
			Env  []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"Env"`
		}
		if err := json.Unmarshal(stacksBytes, &stacks); err != nil {
			return fmt.Errorf("failed to parse stacks: %w", err)
		}
		var stackSpec *struct {
			ID   int    `json:"Id"`
			Name string `json:"Name"`
			Env  []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"Env"`
		}
		for i := range stacks {
			if stacks[i].Name == stackName {
				stackSpec = &stacks[i]
				break
			}
		}
		if stackSpec == nil {
			return fmt.Errorf("stack %q not found", stackName)
		}

		// standalone: pouze update stack_env_var v env + pullImage=true, prune=true
		if updateRevision {
			sfURL := fmt.Sprintf("%s/stacks/%d/file", client.Endpoint, stackSpec.ID)
			sfBytes, code, err := apiGETWithCode(sfURL, client.APIKey, client)
			if err != nil || code != 200 {
				return fmt.Errorf("failed to read stack file (%d): %w", code, err)
			}
			var sf struct {
				StackFileContent string `json:"StackFileContent"`
			}
			if err := json.Unmarshal(sfBytes, &sf); err != nil {
				return fmt.Errorf("failed to parse stack file content: %w", err)
			}

			// ensure/update stack_env_var in Env
			env := stackSpec.Env
			found := false
			for i := range env {
				if env[i].Name == stackEnvVar {
					env[i].Value = revision
					found = true
					break
				}
			}
			if !found {
				env = append(env, struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				}{Name: stackEnvVar, Value: revision})
			}

			// Build payload (standalone uses different json keys)
			type envKV struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			}
			envPayload := make([]envKV, 0, len(env))
			for _, kv := range env {
				envPayload = append(envPayload, envKV{Name: kv.Name, Value: kv.Value})
			}

			putBody := map[string]interface{}{
				"env":              envPayload,
				"prune":            true,
				"pullImage":        true,
				"stackFileContent": sf.StackFileContent,
			}
			body, _ := json.Marshal(putBody)
			updURL := fmt.Sprintf("%s/stacks/%d?endpointId=%d", client.Endpoint, stackSpec.ID, endpointID)
			resp, code, err := apiPUTWithCode(updURL, client.APIKey, client, body)
			if err != nil || code != 200 {
				return fmt.Errorf("failed to update stack (standalone) (status %d): %s", code, string(resp))
			}
			out.WriteString(fmt.Sprintf("Standalone stack %q updated with %s=%q\n", stackName, stackEnvVar, revision))
		} else {
			out.WriteString("Standalone mode — update_revision=false, nothing to update.\n")
		}
	}

	// Save output and ID
	d.Set("output", out.String())
	d.SetId(fmt.Sprintf("deploy-%d", time.Now().Unix()))
	return nil
}

func resourceDeployRead(d *schema.ResourceData, meta interface{}) error {
	// Stateless resource: nothing to read/refresh that makes sense.
	return nil
}

func resourceDeployDelete(d *schema.ResourceData, meta interface{}) error {
	// Stateless effect; nothing to delete.
	d.SetId("")
	return nil
}
