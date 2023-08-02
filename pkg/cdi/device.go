package cdi

import (
	"fmt"

	"github.com/container-orchestrated-devices/container-device-interface/internal/validation" // to be changed
	"github.com/container-orchestrated-devices/container-device-interface/pkg/parser"          //
	cdi "github.com/container-orchestrated-devices/container-device-interface/specs-go"        //
	oci "github.com/opencontainers/runtime-spec/specs-go"
)

// Device represents a CDI device of a Spec.
type Device struct {
	*cdi.Device
	spec *Spec
}

// Create a new Device, associate it with the given Spec.
func newDevice(spec *Spec, d cdi.Device) (*Device, error) {
	dev := &Device{
		Device: &d,
		spec:   spec,
	}

	if err := dev.validate(); err != nil {
		return nil, err
	}

	return dev, nil
}

//	GetSpec return the Spec this device is denied in.
//
// GetSpec returns the Spec this device is defined in.
func (d *Device) GetSpec() *Spec {
	return d.spec
}

// GetQualifiedName returns the qualified name for this device.
func (d *Device) GetQualifiedName() string {
	return parser.QualifiedName(d.spec.GetVendor(), d.spec.GetClass(), d.Name)
}

// ApplyEdits applies the device-speific container edits to an OCI Spec.
func (d *Device) ApplyEdits(ociSpec *oci.Spec) error {
	return d.edits().Apply(ociSpec)
}

// edits returns the applicable container edits for this spec.
func (d *Device) edits() *ContainerEdits {
	return &ContainerEdits{&d.ContainerEdits}
}

// Validate the device.
func (d *Device) validate() error {
	if err := ValidateDeviceName(d.Name); err != nil {
		return err
	}
	name := d.Name
	if d.spec != nil {
		name = d.GetQualifiedName()
	}
	if err := validation.ValidateSpecAnnotations(name, d.Annotations); err != nil {
		return err
	}
	edits := d.edits()
	if edits.isEmpty() {
		return fmt.Errorf("invalid device, empty device edits")
	}
	if err := edits.Validate(); err != nil {
		return fmt.Errorf("invalid device %q: %w", d.Name, err)
	}
	return nil
}
