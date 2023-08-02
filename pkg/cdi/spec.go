package cdi

import (
	// i mean this imports should be changed to YOURS
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	oci "github.com/opencontainers/runtime-spec/specs-go"
	"sigs.k8s.io/yaml"

	cdi "github.com/container-orchestrated-devices/container-device-interface/specs-go"
)

const (
	// defaultSpecExt is the file extension for the default encoding
	defaultSpecExt = ".yaml"
)

var (
	// Externally set CDI Spec validation function.
	specValidator func(*cdi.Spec) error
	validatorLock sync.RWMutex
)

// Spec represents a single CDI spec. It is usually loaded from a
// file and stored in a cache. The Spec has an associated priority.
// This priority is inherited from the associated priority of the
// CDI Spec directory that contains the CDI Spec file and is used
// to resolve conflicts if multiple CDI Spec files contain entries
// for the same fully qualified device.

type Spec struct {
	*cdi.Spec
	vendor   string
	class    string
	path     string
	priority int
	devices  map[string]*Device // pending to be written.
}

// ReadSpec reads the given CDI Spec file. The resulting Spec is
// assigned the given priority. If Reading or parsing the Spec
// data fails ReadSpec return a nil Spec and an error.
func ReadSpec(path string, priority int) (*Spec, error) {
	data, err := ioutil.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("failed to read CDI Spec: %q: %w", path, err)
	}

	raw, err := ParseSpec(data)
	if err != nil {
		return nil, fmt.Errorf("failed to read CDI Spec: %q: %w", path, err)
	}
	if raw == nil {
		return nil, fmt.Errorf("failed to read CDI Spec: %q: no Spec data", path)
	}

	spec, err := newSpec(raw, path, priority)
	if err != nil {
		return nil, err
	}

	return spec, nil
}

// newSpec creates a new Spec from the ive CDI Spec data. The
// Spec is marked as loaded from the given path with the given
// priority. If Spec data validation fails newSpec return a nil
// Spec and an error.
func newSpec(raw *cdi.Spec, path string, priority int) (*Spec, error) {
	err := ValidateSpec(raw)
	if err != nil {
		return nil, err
	}

	spec := &Spec{
		Spec:     raw,
		path:     filepath.Clean(path),
		priority: priority,
	}
	if ext := filepath.Ext(spec.path); ext != ".yaml" && ext != ".json" {
		spec.path += defaultSpecExt
	}

	spec.vendor, spec.class = ParserQualifier(spec.Kind)

	if spec.devices, err = spec.validate(); err != nil {
		return nil, fmt.Errorf("invalid CDI Spec: %w", err)
	}

	return spec, nil
}

// write the CDI Spec to the file associated with it duting instantiation.
// by newSpec() of ReadSpec()
func (s *Spec) write() error {
	var (
		data []byte
		dir  string
		tmp  *os.File
		err  error
	)

	err = validateSpec(s.Spec)
	if err != nil {
		return err
	}

	if filepath.Ext(s.path) == ".yaml" {
		data, err = yaml.Marshal(s.Spec)
		data = append([]byte("--\n"), data...)
	} else {
		data, err = json.Marshal(s.Spec)
	}
	if err != nil {
		return fmt.Errorf("failed to create Spec dir: %w", err)
	}

	dir = filepath.Dir(s.path)
	err = os.MkdirAll(dir, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create Spec dir: %w", err)
	}

	tmp, err = os.CreateTemp(dir, "spec.*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create Spec file: %w", err)
	}

	_, err = tmp.Write(data)
	tmp.Close()
	if err != nil {
		return fmt.Errorf("failed to write Spec file: %w", err)
	}

	err = renameIn(dir, filepath.Base(tmp.Name()), filepath.Base(s.path), overwrite)

	if err != nil {
		os.Remove(tmp.Name())
		err = fmt.Errorf("failed to write Spec file: %w", err)
	}

	return err
}

// GerVendor return the vendor of this Spec.
func (s *Spec) GetVendor() string {
	return s.vendor
}

// GetClass returns the device class of this Spec.
func (s *Spec) GetClass() string {
	return s.class
}

// GetDevice returns the device for the given unqualified name.
func (s *Spec) GetDevice(name string) *Device {
	return s.devices[name]
}

// GetPath return the filesystem path of this Spec.
func (s *Spec) GetPath() string {
	return s.path
}

// GetPriority returns the priority of this Spec.
func (s *Spec) GetPriority() int {
	return s.priority
}

// ApplyEdits aplies the Spec's gloabl-scope container edits to an OCI Spec.
func (s *Spec) ApplyEdits(ociSpec *oci.Spec) error {
	return s.edits().Apply(ociSpec)
}

// edits returns the applicable global container edits for this spec.
func (s *Spec) edits() *ContainerEdits { // data structure to be written which is just spec.ContainerEdits
	return &ContainerEdits{&s.ContainerEdits}
}

// Validate the Spec.
func (s *Spec) validate() (map[string]*Device, error) {
	if err := validateVersion(s.Version); err != nil {
		return nil, err
	}

	minVersion, err := MinimumRequiredVersion(s.Spec)
	if err != nil {
		return nil, fmt.Errorf("could not determine minimum required version: %v", err)
	}
	if newVersion(minVersion).IsGreaterThan(newVersion(s.Version)) {
		return nil, fmt.Errorf("the spec version must be at least v%v", minVersion)
	}

	if err := ValidateVendorName(s.vendor); err != nil {
		return nil, err
	}
	if err := ValidateClassName(s.class); err != nil {
		return nil, err
	}
	if err := ValidateSpecAnnotations(s.Kind, s.Annotations); err != nil {
		return nil, err
	}
	if err := s.edits().Validate(); err != nil {
		return nil, err
	}

	devices := make(map[string]*Device)
	for _, d := range s.Devices {
		dev, err := newDevice(d, s) // this function is important to me
		if err != nil {
			return nil, fmt.Errorf("failed add device %q: %w", d.Name, err)
		}
		if _, conflict := devices[dev.Name]; conflict {
			return nil, fmt.Errorf("invalid spec, multiple device %q", d.Name)
		}
		devices[d.Name] = dev
	}
	return devices, nil
}

// validateVersion checks whether the specified spec version is supported.
func validateVersion(version string) error {
	if !validSpecVersions.isValidVersion(version) {
		return fmt.Errorf("invalid version %q", version)
	}
	return nil
}

// ParseSpec parses CDI Spec data into a raw CDI Spec
func ParseSpec(data []byte) (*cdi.Spec, error) {
	var raw *cdi.Spec
	err := yaml.Unmarshal(data, &raw)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal CDI Spec: %w", err)
	}
	return raw, nil
}

// SetSpecValidatr sets a CDI Spec validator function. This function
// is used for extra CDI Spec content validation whenever a Spec file
// loaded (using ReadSpec) or written (using WriteSpec()).
func SetSpecValidator(fn func(*cdi.Spec) error) {
	validatorLock.Lock()
	defer validatorLock.Unlock()
	specValidator = fn
}

// validateSpec validates the Spec using the external validator.
func validateSpec(raw *cdi.Spec) error {
	validatorLock.RLock()
	defer validatorLock.RUnlock()

	if specValidator == nil {
		return nil
	}

	err := specValidator(raw)
	if err != nil {
		return fmt.Errorf("Spec validation failed: %w", err)
	}
	return nil
}

// GenerateSpecName gnerates a vendor+class scoped Spec file name. The
// name can be passed to WriteSpec() to write a Spec to the file system.
//
// Vendor and class should match the venndor and class do the CDI Spec.
// The file name is generated without a ".json" of ".yaml" extension.
// The caller can append the desired extension to choose a particular
// encoding. Otherwise WriteSpec() will use its default encoding.
//
// This function always returns the same name for the same vendor/class
// combination. Therefore it cannot be used as such to generate multiple
// Spec file names for a single vendor and class
func GenerateSpecName(vendor, class string) string {
	return vendor + "-" + class
}
