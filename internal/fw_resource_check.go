package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource              = (*checkResource)(nil)
	_ resource.ResourceWithConfigure = (*checkResource)(nil)
)

func newCheckResource() resource.Resource { return &checkResource{} }

type checkResource struct{ client *APIClient }

type checkResourceModel struct {
	ID                types.String `tfsdk:"id"`
	EndpointID        types.Int64  `tfsdk:"endpoint_id"`
	StackName         types.String `tfsdk:"stack_name"`
	Revision          types.String `tfsdk:"revision"`
	ServicesList      types.String `tfsdk:"services_list"`
	DesiredState      types.String `tfsdk:"desired_state"`
	Wait              types.Int64  `tfsdk:"wait"`
	WaitBetweenChecks types.Int64  `tfsdk:"wait_between_checks"`
	MaxRetries        types.Int64  `tfsdk:"max_retries"`
	Output            types.String `tfsdk:"output"`
}

func (r *checkResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_check"
}

func (r *checkResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replaceInt := []planmodifier.Int64{int64planmodifier.RequiresReplace()}
	replaceStr := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the check run.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"endpoint_id": schema.Int64Attribute{
				Required:      true,
				Description:   "Identifier of the Portainer endpoint hosting the stack to check.",
				PlanModifiers: replaceInt,
			},
			"stack_name": schema.StringAttribute{
				Required:      true,
				Description:   "Name of the Portainer stack whose service status is being validated.",
				PlanModifiers: replaceStr,
			},
			"revision": schema.StringAttribute{
				Required:      true,
				Description:   "Expected revision (image tag) of running containers/services.",
				PlanModifiers: replaceStr,
			},
			"services_list": schema.StringAttribute{
				Required:      true,
				Description:   "Comma-separated list of service names (without stack prefix).",
				PlanModifiers: replaceStr,
			},
			"desired_state": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Default:       stringdefault.StaticString("running"),
				Description:   "Desired container state (e.g. running).",
				PlanModifiers: replaceStr,
			},
			"wait": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Default:       int64default.StaticInt64(30),
				Description:   "Initial wait before the first check (seconds).",
				PlanModifiers: replaceInt,
			},
			"wait_between_checks": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Default:       int64default.StaticInt64(30),
				Description:   "Wait time between retry checks (seconds).",
				PlanModifiers: replaceInt,
			},
			"max_retries": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Default:       int64default.StaticInt64(3),
				Description:   "Maximum retries for each service check.",
				PlanModifiers: replaceInt,
			},
			"output": schema.StringAttribute{
				Computed:      true,
				Description:   "Human-readable summary of the check results returned by the provider.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *checkResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *checkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan checkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := checkRun(ctx, r.client, &plan, nil); err != nil {
		resp.Diagnostics.AddError("Check failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// checkRun waits for the services to reach the revision and state, writing
// each step to the output (and to progress, when set); it is the operation of
// both the resource and the portainer_check action.
func checkRun(ctx context.Context, client *APIClient, plan *checkResourceModel, progress io.Writer) error {
	endpointID := int(plan.EndpointID.ValueInt64())
	stackName := plan.StackName.ValueString()
	revision := plan.Revision.ValueString()
	desiredState := strings.ToLower(plan.DesiredState.ValueString())
	wait := int(plan.Wait.ValueInt64())
	waitBetween := int(plan.WaitBetweenChecks.ValueInt64())
	maxRetries := int(plan.MaxRetries.ValueInt64())

	shortServices := splitAndTrimCSV(plan.ServicesList.ValueString())
	fullServices := make([]string, 0, len(shortServices))
	for _, s := range shortServices {
		fullServices = append(fullServices, fmt.Sprintf("%s_%s", stackName, s))
	}

	var out strings.Builder
	var w io.Writer = &out
	if progress != nil {
		w = io.MultiWriter(&out, progress)
	}
	fmt.Fprintf(w, "Starting container check for stack %q with revision %q\n", stackName, revision)

	if wait > 0 {
		fmt.Fprintf(w, "Waiting %d seconds before first check...\n", wait)
		if err := sleepCtx(ctx, time.Duration(wait)*time.Second); err != nil {
			return err
		}
	}

	// Detect Swarm
	swURL := fmt.Sprintf("%s/endpoints/%d/docker/swarm", client.Endpoint, endpointID)
	swBody, swCode, err := apiGETWithCodeCtx(ctx, swURL, client.APIKey, client)
	if err != nil {
		return err
	}
	isSwarm := swCode == 200 && strings.Contains(string(swBody), `"ID"`)

	if isSwarm {
		_, _ = io.WriteString(w, "Docker Swarm detected — using swarm check logic.\n")
		err = checkSwarmServices(ctx, client, endpointID, revision, desiredState, fullServices, maxRetries, waitBetween, w)
	} else {
		_, _ = io.WriteString(w, "Docker Standalone detected — using container check logic.\n")
		err = checkStandaloneContainers(ctx, client, endpointID, revision, desiredState, fullServices, maxRetries, waitBetween, w)
	}
	if err != nil {
		return err
	}

	plan.Output = types.StringValue(out.String())
	plan.ID = types.StringValue(fmt.Sprintf("check-%d", time.Now().Unix()))
	return nil
}

// Read is stateless: the check ran once, at creation.
func (r *checkResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {}

// Update is never called: every argument requires replacement.
func (r *checkResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Checks cannot be updated", "Every argument of portainer_check forces a new resource.")
}

// Delete only forgets the check.
func (r *checkResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

func checkSwarmServices(ctx context.Context, client *APIClient, endpointID int, revision, desiredState string, fullServices []string, maxRetries, waitBetween int, out io.Writer) error {
	imgRe := regexp.MustCompile(`^(.+?):([^@]+)(?:@.*)?$`)

	for _, service := range fullServices {
		success := false
		for attempt := 1; attempt <= maxRetries; attempt++ {
			filter := fmt.Sprintf(`{"service":{"%s":true},"desired-state":{"%s":true}}`, service, desiredState)
			tasksURL := fmt.Sprintf("%s/endpoints/%d/docker/tasks?filters=%s", client.Endpoint, endpointID, url.QueryEscape(filter))
			tasksBody, code, err := apiGETWithCodeCtx(ctx, tasksURL, client.APIKey, client)
			if err != nil {
				return fmt.Errorf("error fetching tasks for %s: %w", service, err)
			}
			if code != 200 {
				fmt.Fprintf(out, "Attempt %d/%d: failed to fetch tasks (status %d)\n", attempt, maxRetries, code)
				if err := sleepCtx(ctx, time.Duration(waitBetween)*time.Second); err != nil {
					return err
				}
				continue
			}

			var tasks []map[string]interface{}
			if err := json.Unmarshal(tasksBody, &tasks); err != nil {
				return fmt.Errorf("failed to parse tasks JSON for %s: %w", service, err)
			}
			if len(tasks) == 0 {
				fmt.Fprintf(out, "Attempt %d/%d: no tasks found for service %q\n", attempt, maxRetries, service)
				if err := sleepCtx(ctx, time.Duration(waitBetween)*time.Second); err != nil {
					return err
				}
				continue
			}

			okReplicas := 0
			for _, t := range tasks {
				spec := mustMap(t["Spec"])
				cs := mustMap(spec["ContainerSpec"])
				image, _ := cs["Image"].(string)
				taskState, _ := mustMap(t["Status"])["State"].(string)
				state := strings.ToLower(taskState)

				if m := imgRe.FindStringSubmatch(image); len(m) == 3 {
					if m[2] == revision && state == desiredState {
						okReplicas++
					}
				}
			}

			if okReplicas == len(tasks) {
				fmt.Fprintf(out, "Service %q OK — all %d/%d tasks at revision %q and state %q\n",
					service, okReplicas, len(tasks), revision, desiredState)
				success = true
				break
			}
			fmt.Fprintf(out, "Attempt %d/%d: %d/%d tasks match revision %q and state %q\n",
				attempt, maxRetries, okReplicas, len(tasks), revision, desiredState)
			if err := sleepCtx(ctx, time.Duration(waitBetween)*time.Second); err != nil {
				return err
			}
		}
		if !success {
			return fmt.Errorf("service %q did not reach revision %q and state %q after %d retries",
				service, revision, desiredState, maxRetries)
		}
	}
	return nil
}

func checkStandaloneContainers(ctx context.Context, client *APIClient, endpointID int, revision, desiredState string, fullServices []string, maxRetries, waitBetween int, out io.Writer) error {
	for _, service := range fullServices {
		success := false
		for attempt := 1; attempt <= maxRetries; attempt++ {
			containersURL := fmt.Sprintf("%s/endpoints/%d/docker/containers/json?all=1", client.Endpoint, endpointID)
			containersBody, code, err := apiGETWithCodeCtx(ctx, containersURL, client.APIKey, client)
			if err != nil {
				return fmt.Errorf("failed to list containers: %w", err)
			}
			if code != http.StatusOK {
				return fmt.Errorf("failed to list containers: status %d", code)
			}

			var containers []map[string]interface{}
			if err := json.Unmarshal(containersBody, &containers); err != nil {
				return fmt.Errorf("failed to parse containers list: %w", err)
			}

			for _, c := range containers {
				nameList, _ := c["Names"].([]interface{})
				containerState, _ := c["State"].(string)
				state := strings.ToLower(containerState)
				image, _ := c["Image"].(string)
				if len(nameList) == 0 {
					continue
				}
				firstName, _ := nameList[0].(string)
				name := strings.TrimPrefix(firstName, "/")

				fmt.Fprintf(out, "DEBUG: checking container=%q (image=%q, state=%q)\n", name, image, state)

				normalizedName := strings.ReplaceAll(name, "-", "_")
				normalizedService := strings.ReplaceAll(service, "-", "_")

				if strings.Contains(normalizedName, normalizedService) {
					cleanImage := image
					if strings.Contains(image, "@") {
						cleanImage = strings.Split(image, "@")[0]
					}
					if strings.HasSuffix(cleanImage, ":"+revision) && state == desiredState {
						fmt.Fprintf(out, "Container %q OK — revision %q, state %q\n", name, revision, desiredState)
						success = true
						break
					}
				}
			}

			if success {
				break
			}
			fmt.Fprintf(out, "Attempt %d/%d: container %q not yet matching desired revision/state\n", attempt, maxRetries, service)
			if err := sleepCtx(ctx, time.Duration(waitBetween)*time.Second); err != nil {
				return err
			}
		}

		if !success {
			return fmt.Errorf("container %q is not running in revision %q and state %q after %d retries",
				service, revision, desiredState, maxRetries)
		}
	}
	return nil
}
