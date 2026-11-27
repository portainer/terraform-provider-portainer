package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// portainer_docker_volume creates a Docker volume on an environment. Every
// attribute forces a new volume.

var newDockerVolumeResource = dkNewResource(&dkResourceDef{
	typeSuffix: "_docker_volume",
	schema:     dockerVolumeSchema,
	create:     dockerVolumeCreate,
	read:       dockerVolumeRead,
	delete:     dockerVolumeDelete,
	importer:   dockerVolumeImport,
	identity:   dockerVolumeIdentity,
})

func dockerVolumeSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":                  schema.StringAttribute{Computed: true, Description: "Identifier of the resource.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"driver":              schema.StringAttribute{Optional: true, Computed: true, Description: "Driver used by the Docker volume (e.g., local, nfs, btrfs). Left unset, the engine's default driver is used: `local` on Docker, `d2k` on a D2K environment.", PlanModifiers: []planmodifier.String{dkReplaceString(), stringplanmodifier.UseStateForUnknown()}},
			"driver_opts":         schema.MapAttribute{ElementType: types.StringType, Optional: true, Description: "Driver-specific options passed to the Docker volume driver.", PlanModifiers: []planmodifier.Map{dkReplaceMap()}},
			"endpoint_id":         schema.Int64Attribute{Required: true, Description: "ID of the Portainer environment (Docker host or Swarm) where the volume is created.", PlanModifiers: []planmodifier.Int64{dkReplaceInt64()}},
			"labels":              schema.MapAttribute{ElementType: types.StringType, Optional: true, Description: "Key/value labels attached to the Docker volume.", PlanModifiers: []planmodifier.Map{dkReplaceMap()}},
			"name":                schema.StringAttribute{Required: true, Description: "Name of the Docker volume.", PlanModifiers: []planmodifier.String{dkReplaceString()}},
			"resource_control_id": schema.Int64Attribute{Computed: true, Description: "ID of the Portainer resource control associated with this Docker volume."},
		},
		Blocks: map[string]schema.Block{
			"cluster_volume_spec": schema.ListNestedBlock{
				Description:   "Specification for a Swarm cluster volume (CSI-backed volume managed by Docker Swarm).",
				PlanModifiers: []planmodifier.List{dkReplaceList()},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"availability": schema.StringAttribute{Optional: true, Description: "Availability of the cluster volume (active, pause, or drain)."},
						"group":        schema.StringAttribute{Optional: true, Description: "Group name used to associate related cluster volumes."},
					},
					Blocks: map[string]schema.Block{
						"access_mode": schema.ListNestedBlock{
							Description: "Access mode describing how the cluster volume may be mounted by tasks.",
							Validators:  []validator.List{listvalidator.SizeAtMost(1)},
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"mount_volume": schema.MapAttribute{ElementType: types.StringType, Optional: true, Description: "Filesystem mount options applied when the cluster volume is attached."},
									"scope":        schema.StringAttribute{Optional: true, Description: "Scope of the cluster volume access (e.g., single or multi node)."},
									"sharing":      schema.StringAttribute{Optional: true, Description: "Sharing mode for the cluster volume (e.g., none, readonly, onewriter, all)."},
								},
							},
						},
						"accessibility_requirements": schema.ListNestedBlock{
							Description: "Topology constraints describing where the cluster volume can be accessed.",
							Validators:  []validator.List{listvalidator.SizeAtMost(1)},
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{},
								Blocks: map[string]schema.Block{
									"preferred": schema.ListNestedBlock{
										Description: "Topologies in which the cluster volume should preferably be accessible.",
										NestedObject: schema.NestedBlockObject{
											Attributes: map[string]schema.Attribute{
												"property1": schema.StringAttribute{Optional: true, Description: "First topology key/value pair describing a preferred location."},
												"property2": schema.StringAttribute{Optional: true, Description: "Second topology key/value pair describing a preferred location."},
											},
										},
									},
									"requisite": schema.ListNestedBlock{
										Description: "Topologies in which the cluster volume must be accessible.",
										NestedObject: schema.NestedBlockObject{
											Attributes: map[string]schema.Attribute{
												"property1": schema.StringAttribute{Optional: true, Description: "First topology key/value pair describing a required location."},
												"property2": schema.StringAttribute{Optional: true, Description: "Second topology key/value pair describing a required location."},
											},
										},
									},
								},
							},
						},
						"capacity_range": schema.ListNestedBlock{
							Description: "Capacity bounds requested for the cluster volume.",
							Validators:  []validator.List{listvalidator.SizeAtMost(1)},
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"limit_bytes":    schema.Int64Attribute{Optional: true, Description: "Maximum allowed size of the cluster volume in bytes."},
									"required_bytes": schema.Int64Attribute{Optional: true, Description: "Minimum required size of the cluster volume in bytes."},
								},
							},
						},
						"secrets": schema.ListNestedBlock{
							Description: "Secrets passed to the CSI plugin when provisioning the cluster volume.",
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"key":    schema.StringAttribute{Required: true, Description: "Key used by the CSI plugin to identify the secret value."},
									"secret": schema.StringAttribute{Required: true, Description: "Name of the Docker Swarm secret providing the value."},
								},
							},
						},
					},
				},
			},
		},
	}
}

type DockerVolumeSpec struct {
	Name              string             `json:"Name"`
	Driver            string             `json:"Driver,omitempty"`
	DriverOpts        map[string]string  `json:"DriverOpts,omitempty"`
	Labels            map[string]string  `json:"Labels,omitempty"`
	ClusterVolumeSpec *ClusterVolumeSpec `json:"ClusterVolumeSpec,omitempty"`
}

type ClusterVolumeSpec struct {
	Group                     string                     `json:"Group,omitempty"`
	AccessMode                *AccessMode                `json:"AccessMode,omitempty"`
	Secrets                   []ClusterVolumeSecret      `json:"Secrets,omitempty"`
	AccessibilityRequirements *AccessibilityRequirements `json:"AccessibilityRequirements,omitempty"`
	CapacityRange             *CapacityRange             `json:"CapacityRange,omitempty"`
	Availability              string                     `json:"Availability,omitempty"`
}

type AccessMode struct {
	Scope       string            `json:"Scope,omitempty"`
	Sharing     string            `json:"Sharing,omitempty"`
	MountVolume map[string]string `json:"MountVolume,omitempty"`
}

type ClusterVolumeSecret struct {
	Key    string `json:"Key"`
	Secret string `json:"Secret"`
}

type AccessibilityRequirements struct {
	Requisite []map[string]string `json:"Requisite,omitempty"`
	Preferred []map[string]string `json:"Preferred,omitempty"`
}

type CapacityRange struct {
	RequiredBytes int64 `json:"RequiredBytes,omitempty"`
	LimitBytes    int64 `json:"LimitBytes,omitempty"`
}

type dockerVolumeCreateResponse struct {
	Name       string            `json:"Name"`
	Driver     string            `json:"Driver"`
	Labels     map[string]string `json:"Labels"`
	Options    map[string]string `json:"Options"`
	Mountpoint string            `json:"Mountpoint"`
	Portainer  struct {
		ResourceControl struct {
			ID int `json:"Id"`
		} `json:"ResourceControl"`
	} `json:"Portainer"`
}

func dockerVolumeCreate(ctx context.Context, d *dkBag, client *APIClient) error {
	volume := DockerVolumeSpec{
		Name:       dkString(d, "name"),
		Driver:     dkString(d, "driver"),
		DriverOpts: dkStringMap(dkMap(d, "driver_opts")),
		Labels:     dkStringMap(dkMap(d, "labels")),
	}

	if rawSpec, ok := d.GetOk("cluster_volume_spec"); ok {
		specList := rawSpec.([]interface{})
		if len(specList) > 0 && specList[0] != nil {
			volume.ClusterVolumeSpec = expandClusterVolumeSpec(specList[0].(map[string]interface{}))
		}
	}

	endpointID := dkInt(d, "endpoint_id")
	reqURL := fmt.Sprintf("%s/endpoints/%d/docker/volumes/create", client.Endpoint, endpointID)

	var response dockerVolumeCreateResponse

	if err := doJSON(ctx, client, http.MethodPost, reqURL, volume, &response); err != nil {
		return fmt.Errorf("failed to create volume: %w", err)
	}

	name := response.Name
	if name == "" {
		name = volume.Name
	}

	d.SetID(fmt.Sprintf("%d:%s", endpointID, name))
	if response.Driver != "" {
		if err := d.Set("driver", response.Driver); err != nil {
			return err
		}
	}

	if response.Portainer.ResourceControl.ID != 0 {
		if err := d.Set("resource_control_id", response.Portainer.ResourceControl.ID); err != nil {
			return err
		}
	}

	return nil
}

func dockerVolumeRead(ctx context.Context, d *dkBag, client *APIClient) error {
	endpointID := dkInt(d, "endpoint_id")
	name := dkString(d, "name")

	reqURL := fmt.Sprintf("%s/endpoints/%d/docker/volumes/%s", client.Endpoint, endpointID, url.PathEscape(name))

	var result struct {
		Name       string            `json:"Name"`
		Driver     string            `json:"Driver"`
		Labels     map[string]string `json:"Labels"`
		Options    map[string]string `json:"Options"`
		Mountpoint string            `json:"Mountpoint"`
		Portainer  struct {
			ResourceControl struct {
				ID int `json:"Id"`
			} `json:"ResourceControl"`
		} `json:"Portainer"`
	}

	if err := doJSON(ctx, client, http.MethodGet, reqURL, nil, &result); err != nil {
		if isAPINotFound(err) {
			d.SetID("")
			return nil
		}
		return fmt.Errorf("failed to read volume: %w", err)
	}

	if err := d.SetFields(map[string]interface{}{
		"name":        result.Name,
		"driver":      result.Driver,
		"labels":      result.Labels,
		"driver_opts": result.Options,
	}); err != nil {
		return err
	}

	d.SetID(fmt.Sprintf("%d:%s", endpointID, result.Name))

	if result.Portainer.ResourceControl.ID != 0 {
		if err := d.Set("resource_control_id", result.Portainer.ResourceControl.ID); err != nil {
			return err
		}
	}

	return nil
}

func dockerVolumeDelete(ctx context.Context, d *dkBag, client *APIClient) error {
	endpointID := dkInt(d, "endpoint_id")
	name := dkString(d, "name")

	reqURL := fmt.Sprintf("%s/endpoints/%d/docker/volumes/%s", client.Endpoint, endpointID, url.PathEscape(name))
	if err := doJSON(ctx, client, http.MethodDelete, reqURL, nil, nil); err != nil && !isAPINotFound(err) {
		return fmt.Errorf("failed to delete volume: %w", err)
	}

	d.SetID("")
	return nil
}

func expandClusterVolumeSpec(m map[string]interface{}) *ClusterVolumeSpec {
	return &ClusterVolumeSpec{
		Group:        m["group"].(string),
		Availability: m["availability"].(string),
	}
}

// dockerVolumeImport takes "<endpoint_id>:<volume_name>", or the "-" form
// from before v3.0.0.
func dockerVolumeImport(_ context.Context, d *dkBag, _ *APIClient) error {
	endpointID, name, err := splitEndpointImportID(d.ID(), "<endpoint_id>:<volume_name>")
	if err != nil {
		return err
	}
	if err := d.Set("endpoint_id", endpointID); err != nil {
		return err
	}
	if err := d.Set("name", name); err != nil {
		return err
	}
	d.SetID(fmt.Sprintf("%d:%s", endpointID, name))
	return nil
}
