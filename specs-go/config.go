package specs

import "os"

// Version of the spec. Putting the same as Nvidia's for now.

const CurrentVersion = "0.6.0"

// Spec is the base configuration for CDI

type Spec struct {
	Version string `json:"cdiVersion"`
	Kind    string `json:"kind"`
	// Annotation add metadata per CDI spec. Note that these are CDI specific and do not affect container metadata.
	Annotations    map[string]string `json:"annotations,omitempty"`
	Devices        []Device          `json:"devices"`
	ContainerEdits ContainerEdits    `json:"containerEdits,omitempty"`
}

// Device is a "Device" a container runtime can add to a container

type Device struct {
	Name string `json:"name"`
	// Annotation add metadata per Device. Note that these are CDI specific and do not affect container metadata.
	Annotations map[string]string `json:"annotations,omitempty"`
	// ContainerEdits are the edits to the OCI spec that are required for this device
	ContainerEdits ContainerEdits `json:"containerEdits"`
}

// ContainerEdits are edits a container runtime must make to the OCI spec to expose the device
type ContainerEdits struct {
	Env         []string      `json:"env,omitempty"`
	DeviceNodes []*DeviceNode `json:"deviceNodes,omitempty"`
	Hooks       []*Hook       `json:"hooks,omitempty"`
	Mounts      []*Mount      `json:"mounts,omitempty"`
}

// DeviceNode represents a device node that needs to be added to the OCI spec
type DeviceNode struct {
	// Path is the path to the device node in the container
	Path     string `json:"path"`
	HostPath string `json:"hostPath,omitempty"`
	// Major is the major number of the device node
	Major int64 `json:"major,omitempty"`
	// Minor is the minor number of the device node
	Minor int64 `json:"minor,omitempty"`
	// FileMode is the file mode of the device node
	FileMode *os.FileMode `json:"fileMode,omitempty"`
	// UID is the user id of the device node
	UID *uint32 `json:"uid,omitempty"`
	// GID is the group id of the device node
	GID         *uint32 `json:"gid,omitempty"`
	Type        string  `json:"type,omitempty"`
	Permissions string  `json:"permissions,omitempty"`
}

// Mount represents a mount that needs to be added to the OCI spec.
type Mount struct {
	HostPath      string   `json:"hostPath,omitempty"`
	ContainerPath string   `json:"containerPath,omitempty"`
	Options       []string `json:"options,omitempty"`
	Type          string   `json:"type,omitempty"`
}

type Hook struct {
	HookName string   `json:"hookName"`
	Path     string   `json:"path"`
	Args     []string `json:"args,omitempty"`
	Env      []string `json:"env,omitempty"`
	Timeout  *int     `json:"timeout,omitempty"`
}
