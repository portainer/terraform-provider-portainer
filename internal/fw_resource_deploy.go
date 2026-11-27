package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// portainer_deploy rolls a new image revision out to the services of a stack
// (and optionally into one of its environment variables). It is a one-shot
// action: Read and Delete do nothing.

var newDeployResource = dkNewResource(&dkResourceDef{
	typeSuffix: "_deploy",
	schema:     deploySchema,
	create:     deployCreate,
	read:       deployRead,
	delete:     deployDelete,
	timeouts:   map[string]time.Duration{"create": 15 * time.Minute},
})

func deploySchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":              schema.StringAttribute{Computed: true, Description: "Identifier of the resource.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"endpoint_id":     schema.Int64Attribute{Required: true, Description: "Identifier of the Portainer endpoint hosting the stack to deploy/update.", PlanModifiers: []planmodifier.Int64{dkReplaceInt64()}},
			"force_update":    schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "If true, call Portainer forceupdateservice endpoint for each updated service (after optional wait).", PlanModifiers: []planmodifier.Bool{dkReplaceBool()}},
			"output":          schema.StringAttribute{Computed: true, Description: "Human-readable summary of the deployment actions performed by the provider."},
			"revision":        schema.StringAttribute{Required: true, Description: "Target image tag/revision to set on services and optionally on stack ENV in stack_env_var.", PlanModifiers: []planmodifier.String{dkReplaceString()}},
			"services_list":   schema.StringAttribute{Required: true, Description: "Comma-separated list of service names (without stack prefix).", PlanModifiers: []planmodifier.String{dkReplaceString()}},
			"stack_env_var":   schema.StringAttribute{Required: true, Description: "Name of stack environment variable to update.", PlanModifiers: []planmodifier.String{dkReplaceString()}},
			"stack_name":      schema.StringAttribute{Required: true, Description: "Name of the Portainer stack whose services are being deployed.", PlanModifiers: []planmodifier.String{dkReplaceString()}},
			"update_revision": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), Description: "If true, also update stack ENV variable in stack_env_var to the provided revision.", PlanModifiers: []planmodifier.Bool{dkReplaceBool()}},
			"wait":            schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(30), Description: "Seconds to wait before force-updating a service (only when force_update = true).", PlanModifiers: []planmodifier.Int64{dkReplaceInt64()}},
		},
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{Create: true}),
		},
	}
}

func deployCreate(ctx context.Context, d *dkBag, client *APIClient) error {
	timeout := d.Timeout("create", 15*time.Minute)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	stackEnvVar := dkString(d, "stack_env_var")
	endpointID := dkInt(d, "endpoint_id")
	stackName := dkString(d, "stack_name")
	revision := dkString(d, "revision")
	updateRevision := dkBool(d, "update_revision")
	forceUpdate := dkBool(d, "force_update")
	wait := dkInt(d, "wait")

	servicesListRaw := dkString(d, "services_list")
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
	swBody, swCode, err := apiGETWithCodeCtx(ctx, swURL, client.APIKey, client)
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
		stacksBytes, err := apiGETRaw(ctx, client, stacksURL)
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
		servicesBytes, err := apiGETRaw(ctx, client, servicesURL)
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
				fmt.Fprintf(&out, "Service %q already uses revision %q — skip.\n", svcName, revision)
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
			respBytes, code, err := apiPOSTWithCodeCtx(ctx, postURL, client.APIKey, client, postBody)
			if err != nil {
				return fmt.Errorf("service %s update request failed: %w", svcName, err)
			}
			if code != 200 {
				return fmt.Errorf("service %s update failed: status %d, body: %s", svcName, code, string(respBytes))
			}
			updatedAny = true
			fmt.Fprintf(&out, "Service %q updated to %q\n", svcName, newImage)

			// check warnings
			var updOut struct {
				Warnings interface{} `json:"Warnings"`
			}
			_ = json.Unmarshal(respBytes, &updOut)
			if updOut.Warnings != nil && fmt.Sprint(updOut.Warnings) != "<nil>" && fmt.Sprint(updOut.Warnings) != "None" {
				fmt.Fprintf(&out, "WARN: service update returned warnings: %v\n", updOut.Warnings)
			}

			// optional force update
			if forceUpdate {
				if wait > 0 {
					if err := sleepCtx(ctx, time.Duration(wait)*time.Second); err != nil {
						return err
					}
				}
				forceURL := fmt.Sprintf("%s/endpoints/%d/forceupdateservice", client.Endpoint, endpointID)
				forcePayload := map[string]interface{}{
					"pullImage": true,
					"serviceID": svcName,
				}
				body, _ := json.Marshal(forcePayload)
				resp, code, err := apiPUTWithCodeCtx(ctx, forceURL, client.APIKey, client, body)
				if err != nil || code != 200 {
					fmt.Fprintf(&out, "Force update of %q failed (status %d): %s\n", svcName, code, string(resp))
				} else {
					fmt.Fprintf(&out, "Force update of %q succeeded\n", svcName)
				}
			}
		}

		if !updatedAny {
			fmt.Fprintf(&out, "No update needed. All requested services already use revision %q.\n", revision)
		}

		// Update stack_env_var env on stack (if requested)
		if updateRevision && stackSpec != nil {
			// GET stack file
			sfURL := fmt.Sprintf("%s/stacks/%d/file", client.Endpoint, stackSpec.ID)
			sfBytes, code, err := apiGETWithCodeCtx(ctx, sfURL, client.APIKey, client)
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
			// Portainer 2.45 deploys asynchronously and rejects an update while a
			// deployment runs (issue #145); wait for it before and after ours,
			// and report ours if it failed.
			stackID := strconv.Itoa(stackSpec.ID)
			before, err := waitForStackIdle(ctx, client, stackID)
			if err != nil {
				return err
			}
			updURL := fmt.Sprintf("%s/stacks/%d?endpointId=%d", client.Endpoint, stackSpec.ID, endpointID)
			resp, code, err := apiPUTWithCodeCtx(ctx, updURL, client.APIKey, client, body)
			if err != nil || code != 200 {
				return fmt.Errorf("failed to update stack %s (status %d): %s", stackEnvVar, code, string(resp))
			}
			if err := waitForStackRedeployed(ctx, client, stackID, before); err != nil {
				return err
			}
			fmt.Fprintf(&out, "Stack %q %s updated to %q\n", stackName, stackEnvVar, revision)
		}

	} else {
		// Standalone
		out.WriteString("Docker Standalone detected — using standalone stack update logic.\n")

		// list stacks and find by name
		stacksURL := fmt.Sprintf("%s/stacks", client.Endpoint)
		stacksBytes, err := apiGETRaw(ctx, client, stacksURL)
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
			sfBytes, code, err := apiGETWithCodeCtx(ctx, sfURL, client.APIKey, client)
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
			// Portainer 2.45 deploys asynchronously and rejects an update while a
			// deployment runs (issue #145); wait for it before and after ours,
			// and report ours if it failed.
			stackID := strconv.Itoa(stackSpec.ID)
			before, err := waitForStackIdle(ctx, client, stackID)
			if err != nil {
				return err
			}
			updURL := fmt.Sprintf("%s/stacks/%d?endpointId=%d", client.Endpoint, stackSpec.ID, endpointID)
			resp, code, err := apiPUTWithCodeCtx(ctx, updURL, client.APIKey, client, body)
			if err != nil || code != 200 {
				return fmt.Errorf("failed to update stack (standalone) (status %d): %s", code, string(resp))
			}
			if err := waitForStackRedeployed(ctx, client, stackID, before); err != nil {
				return err
			}
			fmt.Fprintf(&out, "Standalone stack %q updated with %s=%q\n", stackName, stackEnvVar, revision)
		} else {
			out.WriteString("Standalone mode — update_revision=false, nothing to update.\n")
		}
	}

	// Save output and ID
	if err := d.Set("output", out.String()); err != nil {
		return err
	}
	d.SetID(fmt.Sprintf("deploy-%d", time.Now().Unix()))
	return nil
}

func deployRead(ctx context.Context, d *dkBag, client *APIClient) error {
	// Stateless resource: nothing to read/refresh that makes sense.
	return nil
}

func deployDelete(ctx context.Context, d *dkBag, client *APIClient) error {
	// Stateless effect; nothing to delete.
	d.SetID("")
	return nil
}
