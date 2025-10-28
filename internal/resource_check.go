package internal

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceCheck() *schema.Resource {
	return &schema.Resource{
		Create: resourceCheckCreate,
		Read:   resourceCheckRead,
		Delete: resourceCheckDelete,
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
			"revision": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Expected revision (image tag) of running containers/services.",
			},
			"services_list": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Comma-separated list of service names (without stack prefix).",
			},
			"desired_state": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "running",
				ForceNew:    true,
				Description: "Desired container state (e.g. running).",
			},
			"wait": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     30,
				ForceNew:    true,
				Description: "Initial wait before the first check (seconds).",
			},
			"wait_between_checks": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     30,
				ForceNew:    true,
				Description: "Wait time between retry checks (seconds).",
			},
			"max_retries": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     3,
				ForceNew:    true,
				Description: "Maximum retries for each service check.",
			},
			"output": {
				Type:     schema.TypeString,
				Computed: true,
				ForceNew: true,
			},
		},
	}
}

func resourceCheckCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	endpointID := d.Get("endpoint_id").(int)
	stackName := d.Get("stack_name").(string)
	revision := d.Get("revision").(string)
	desiredState := strings.ToLower(d.Get("desired_state").(string))
	wait := d.Get("wait").(int)
	waitBetween := d.Get("wait_between_checks").(int)
	maxRetries := d.Get("max_retries").(int)

	servicesRaw := d.Get("services_list").(string)
	shortServices := splitAndTrimCSV(servicesRaw)
	fullServices := make([]string, 0, len(shortServices))
	for _, s := range shortServices {
		fullServices = append(fullServices, fmt.Sprintf("%s_%s", stackName, s))
	}

	var out strings.Builder
	out.WriteString(fmt.Sprintf("Starting container check for stack %q with revision %q\n", stackName, revision))

	if wait > 0 {
		out.WriteString(fmt.Sprintf("Waiting %d seconds before first check...\n", wait))
		time.Sleep(time.Duration(wait) * time.Second)
	}

	// Detect Swarm
	swURL := fmt.Sprintf("%s/endpoints/%d/docker/swarm", client.Endpoint, endpointID)
	swBody, swCode, err := apiGETWithCode(swURL, client.APIKey, client)
	if err != nil {
		return err
	}
	isSwarm := swCode == 200 && strings.Contains(string(swBody), `"ID"`)

	if isSwarm {
		out.WriteString("Docker Swarm detected — using swarm check logic.\n")
		if err := checkSwarmServices(client, endpointID, revision, desiredState, fullServices, maxRetries, waitBetween, &out); err != nil {
			return err
		}
	} else {
		out.WriteString("Docker Standalone detected — using container check logic.\n")
		if err := checkStandaloneContainers(client, endpointID, revision, desiredState, fullServices, maxRetries, waitBetween, &out); err != nil {
			return err
		}
	}

	d.Set("output", out.String())
	d.SetId(fmt.Sprintf("check-%d", time.Now().Unix()))
	return nil
}

func checkSwarmServices(client *APIClient, endpointID int, revision, desiredState string, fullServices []string, maxRetries, waitBetween int, out *strings.Builder) error {
	imgRe := regexp.MustCompile(`^(.+?):([^@]+)(?:@.*)?$`)

	for _, service := range fullServices {
		success := false
		for attempt := 1; attempt <= maxRetries; attempt++ {
			filter := fmt.Sprintf(`{"service":{"%s":true},"desired-state":{"%s":true}}`, service, desiredState)
			tasksURL := fmt.Sprintf("%s/endpoints/%d/docker/tasks?filters=%s", client.Endpoint, endpointID, url.QueryEscape(filter))
			tasksBody, code, err := apiGETWithCode(tasksURL, client.APIKey, client)
			if err != nil {
				return fmt.Errorf("error fetching tasks for %s: %w", service, err)
			}
			if code != 200 {
				out.WriteString(fmt.Sprintf("Attempt %d/%d: failed to fetch tasks (status %d)\n", attempt, maxRetries, code))
				time.Sleep(time.Duration(waitBetween) * time.Second)
				continue
			}

			var tasks []map[string]interface{}
			if err := json.Unmarshal(tasksBody, &tasks); err != nil {
				return fmt.Errorf("failed to parse tasks JSON for %s: %w", service, err)
			}
			if len(tasks) == 0 {
				out.WriteString(fmt.Sprintf("Attempt %d/%d: no tasks found for service %q\n", attempt, maxRetries, service))
				time.Sleep(time.Duration(waitBetween) * time.Second)
				continue
			}

			okReplicas := 0
			for _, t := range tasks {
				spec := mustMap(t["Spec"])
				cs := mustMap(spec["ContainerSpec"])
				image, _ := cs["Image"].(string)
				state := strings.ToLower(mustMap(t["Status"])["State"].(string))

				if m := imgRe.FindStringSubmatch(image); len(m) == 3 {
					if m[2] == revision && state == desiredState {
						okReplicas++
					}
				}
			}

			if okReplicas == len(tasks) {
				out.WriteString(fmt.Sprintf("Service %q OK — all %d/%d tasks at revision %q and state %q\n",
					service, okReplicas, len(tasks), revision, desiredState))
				success = true
				break
			} else {
				out.WriteString(fmt.Sprintf("Attempt %d/%d: %d/%d tasks match revision %q and state %q\n",
					attempt, maxRetries, okReplicas, len(tasks), revision, desiredState))
				time.Sleep(time.Duration(waitBetween) * time.Second)
			}
		}
		if !success {
			return fmt.Errorf("service %q did not reach revision %q and state %q after %d retries",
				service, revision, desiredState, maxRetries)
		}
	}
	return nil
}

func checkStandaloneContainers(client *APIClient, endpointID int, revision, desiredState string, fullServices []string, maxRetries, waitBetween int, out *strings.Builder) error {

	containersURL := fmt.Sprintf("%s/endpoints/%d/docker/containers/json?all=1", client.Endpoint, endpointID)
	containersBody, code, err := apiGETWithCode(containersURL, client.APIKey, client)
	if err != nil || code != 200 {
		return fmt.Errorf("failed to list containers (status %d): %w", code, err)
	}
	var containers []map[string]interface{}
	if err := json.Unmarshal(containersBody, &containers); err != nil {
		return fmt.Errorf("failed to parse containers list: %w", err)
	}

	for _, service := range fullServices {
		success := false
		for attempt := 1; attempt <= maxRetries; attempt++ {
			containersURL := fmt.Sprintf("%s/endpoints/%d/docker/containers/json?all=1", client.Endpoint, endpointID)
			containersBody, code, err := apiGETWithCode(containersURL, client.APIKey, client)
			if err != nil || code != 200 {
				return fmt.Errorf("failed to list containers (status %d): %w", code, err)
			}

			var containers []map[string]interface{}
			if err := json.Unmarshal(containersBody, &containers); err != nil {
				return fmt.Errorf("failed to parse containers list: %w", err)
			}

			for _, c := range containers {
				nameList, _ := c["Names"].([]interface{})
				state := strings.ToLower(c["State"].(string))
				image, _ := c["Image"].(string)
				if len(nameList) == 0 {
					continue
				}
				name := strings.TrimPrefix(nameList[0].(string), "/")

				out.WriteString(fmt.Sprintf("DEBUG: checking container=%q (image=%q, state=%q)\n", name, image, state))

				normalizedName := strings.ReplaceAll(name, "-", "_")
				normalizedService := strings.ReplaceAll(service, "-", "_")

				if strings.Contains(normalizedName, normalizedService) {
					cleanImage := image
					if strings.Contains(image, "@") {
						cleanImage = strings.Split(image, "@")[0]
					}
					if strings.HasSuffix(cleanImage, ":"+revision) && state == desiredState {
						out.WriteString(fmt.Sprintf("Container %q OK — revision %q, state %q\n", name, revision, desiredState))
						success = true
						break
					}
				}
			}

			if success {
				break
			} else {
				out.WriteString(fmt.Sprintf("Attempt %d/%d: container %q not yet matching desired revision/state\n", attempt, maxRetries, service))
				time.Sleep(time.Duration(waitBetween) * time.Second)
			}
		}

		if !success {
			return fmt.Errorf("container %q is not running in revision %q and state %q after %d retries",
				service, revision, desiredState, maxRetries)
		}
	}
	return nil
}

func resourceCheckRead(d *schema.ResourceData, meta interface{}) error {
	return nil // Stateless
}

func resourceCheckDelete(d *schema.ResourceData, meta interface{}) error {
	d.SetId("")
	return nil
}
